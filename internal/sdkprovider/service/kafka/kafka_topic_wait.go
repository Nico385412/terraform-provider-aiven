package kafka

import (
	"fmt"
	"log"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/semaphore"
)

// kafkaTopicAvailabilityWaiter is used to refresh the Aiven Kafka Topic endpoints when
// provisioning.
type kafkaTopicAvailabilityWaiter struct {
	Client      *aiven.Client
	Project     string
	ServiceName string
	TopicName   string
}

var kafkaTopicAvailabilitySem = semaphore.NewWeighted(1)

func newKafkaTopicAvailabilityWaiter(client *aiven.Client, project, serviceName, topicName string) (*kafkaTopicAvailabilityWaiter, error) {
	if len(project)*len(serviceName)*len(topicName) == 0 {
		return nil, fmt.Errorf("return invalid input: project=%q, serviceName=%q, topicName=%q", project, serviceName, topicName)
	}
	return &kafkaTopicAvailabilityWaiter{
		Client:      client,
		Project:     project,
		ServiceName: serviceName,
		TopicName:   topicName,
	}, nil
}

// RefreshFunc will call the Aiven client and refresh it's state.
// nolint:staticcheck // TODO: Migrate to helper/retry package to avoid deprecated resource.StateRefreshFunc.
func (w *kafkaTopicAvailabilityWaiter) RefreshFunc() resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		cache := getTopicCache()

		// Caching a list of all topics for a service from v1 GET endpoint.
		// Aiven has a request-per-minute limit; therefore, to minimize
		// the request count, we query the V1 list endpoint.
		if len(cache.GetV1List(w.Project, w.ServiceName)) == 0 {
			list, err := w.Client.KafkaTopics.List(w.Project, w.ServiceName)
			if err != nil {
				return nil, "CONFIGURING", fmt.Errorf("error calling v1 list for %s/%s: %w", w.Project, w.ServiceName, err)
			}
			cache.SetV1List(w.Project, w.ServiceName, list)
		}

		// Checking if the topic is in the missing list. If so, trowing 404 error
		if slices.Contains(cache.GetMissing(w.Project, w.ServiceName), w.TopicName) {
			return nil, "CONFIGURING", aiven.Error{Status: 404, Message: fmt.Sprintf("Topic %s is not found", w.TopicName)}
		}

		topic, ok := cache.LoadByTopicName(w.Project, w.ServiceName, w.TopicName)
		if !ok {
			err := w.refresh()

			if err != nil {
				aivenError, ok := err.(aiven.Error)
				if !ok {
					return nil, "CONFIGURING", err
				}

				// Getting topic info can sometimes temporarily fail with 501 and 502. Don't
				// treat that as fatal error but keep on retrying instead.
				if aivenError.Status == 501 || aivenError.Status == 502 {
					log.Printf("[DEBUG] Got an error while waiting for a topic '%s' to be ACTIVE: %s.", w.TopicName, err)
					return nil, "CONFIGURING", nil
				}
				return nil, "CONFIGURING", err
			}

			topic, ok = cache.LoadByTopicName(w.Project, w.ServiceName, w.TopicName)
			if !ok {
				return nil, "CONFIGURING", nil
			}
		}

		log.Printf("[DEBUG] Got `%s` state while waiting for topic `%s` to be up.", topic.State, w.TopicName)

		return topic, topic.State, nil
	}
}

func (w *kafkaTopicAvailabilityWaiter) refresh() error {
	if !kafkaTopicAvailabilitySem.TryAcquire(1) {
		log.Printf("[TRACE] Kafka Topic Availability cache refresh already in progress ...")
		return nil
	}
	defer kafkaTopicAvailabilitySem.Release(1)

	c := getTopicCache()

	// check if topic is already in cache
	if _, ok := c.LoadByTopicName(w.Project, w.ServiceName, w.TopicName); ok {
		return nil
	}

	c.AddToQueue(w.Project, w.ServiceName, w.TopicName)

	for {
		queue := c.GetQueue(w.Project, w.ServiceName)
		if len(queue) == 0 {
			break
		}

		log.Printf("[DEBUG] Kafka Topic queue: %+v", queue)
		v2Topics, err := w.Client.KafkaTopics.V2List(w.Project, w.ServiceName, queue)
		if err != nil {
			// V2 Kafka Topic endpoint retrieves 404 when one or more topics in the batch
			// do not exist but does not say which ones are missing. Therefore, we need to
			// identify the none existing topics.
			if aiven.IsNotFound(err) {
				// If topic is missing in V1 list then it does not exist, flagging it as missing
				for _, t := range queue {
					if !slices.Contains(c.GetV1List(w.Project, w.ServiceName), t) {
						c.DeleteFromQueueAndMarkMissing(w.Project, w.ServiceName, t)
					}
				}
				return nil
			}
			return err
		}

		getTopicCache().StoreByProjectAndServiceName(w.Project, w.ServiceName, v2Topics)
	}

	return nil
}

// Conf sets up the configuration to refresh.
// nolint:staticcheck // TODO: Migrate to helper/retry package to avoid deprecated resource.StateRefreshFunc.
func (w *kafkaTopicAvailabilityWaiter) Conf(timeout time.Duration) *resource.StateChangeConf {
	log.Printf("[DEBUG] Kafka Topic availability waiter timeout %.0f minutes", timeout.Minutes())

	return &resource.StateChangeConf{
		Pending:        []string{"CONFIGURING"},
		Target:         []string{"ACTIVE"},
		Refresh:        w.RefreshFunc(),
		Timeout:        timeout,
		PollInterval:   5 * time.Second,
		NotFoundChecks: 100,
	}
}
