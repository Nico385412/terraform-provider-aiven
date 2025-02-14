package kafka

import (
	"log"
	"sync"

	"github.com/aiven/aiven-go-client"
	"golang.org/x/exp/slices"
)

var (
	once       sync.Once
	topicCache *kafkaTopicCache
)

// kafkaTopicCache represents Kafka Topics cache based on Service and Project identifiers
type kafkaTopicCache struct {
	sync.RWMutex
	internal map[string]map[string]aiven.KafkaTopic
	inQueue  map[string][]string
	missing  map[string][]string
	v1list   map[string][]string
}

// initTopicCache creates new global instance of Kafka Topic Cache
func initTopicCache() {
	log.Print("[DEBUG] Creating an instance of kafkaTopicCache ...")

	once.Do(func() {
		topicCache = &kafkaTopicCache{
			internal: make(map[string]map[string]aiven.KafkaTopic),
			inQueue:  make(map[string][]string),
			missing:  make(map[string][]string),
			v1list:   make(map[string][]string),
		}
	})
}

// getTopicCache gets a global Kafka Topics Cache
func getTopicCache() *kafkaTopicCache {
	return topicCache
}

// LoadByProjectAndServiceName returns a list of Kafka Topics stored in the cache for a given Project
// and Service names, or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (t *kafkaTopicCache) LoadByProjectAndServiceName(projectName, serviceName string) (map[string]aiven.KafkaTopic, bool) {
	t.RLock()
	result, ok := t.internal[projectName+serviceName]
	t.RUnlock()

	return result, ok
}

// LoadByTopicName returns a list of Kafka Topics stored in the cache for a given Project
// and Service names, or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (t *kafkaTopicCache) LoadByTopicName(projectName, serviceName, topicName string) (aiven.KafkaTopic, bool) {
	t.RLock()
	defer t.RUnlock()

	topics, ok := t.internal[projectName+serviceName]
	if !ok {
		return aiven.KafkaTopic{State: "CONFIGURING"}, false
	}

	result, ok := topics[topicName]
	if !ok {
		result.State = "CONFIGURING"
	}

	log.Printf("[TRACE] retrieving from a topic cache `%+#v` for a topic name `%s`", result, topicName)

	return result, ok
}

// DeleteByProjectAndServiceName deletes the cache value for a key which is a combination of Project
// and Service names.
func (t *kafkaTopicCache) DeleteByProjectAndServiceName(projectName, serviceName string) {
	t.Lock()
	delete(t.internal, projectName+serviceName)
	t.Unlock()
}

// StoreByProjectAndServiceName sets the values for a Project name and Service name key.
func (t *kafkaTopicCache) StoreByProjectAndServiceName(projectName, serviceName string, list []*aiven.KafkaTopic) {
	if len(list) == 0 {
		return
	}

	log.Printf("[DEBUG] Updating Kafka Topic cache for project %s and service %s ...", projectName, serviceName)

	for _, topic := range list {
		t.Lock()
		if _, ok := t.internal[projectName+serviceName]; !ok {
			t.internal[projectName+serviceName] = make(map[string]aiven.KafkaTopic)
		}
		t.internal[projectName+serviceName][topic.TopicName] = *topic

		// when topic is added to cache, it need to be deleted from the queue
		for i, name := range t.inQueue[projectName+serviceName] {
			if name == topic.TopicName {
				t.inQueue[projectName+serviceName] = append(t.inQueue[projectName+serviceName][:i], t.inQueue[projectName+serviceName][i+1:]...)
			}
		}

		t.Unlock()
	}
}

// AddToQueue adds a topic name to a queue of topics to be found
func (t *kafkaTopicCache) AddToQueue(projectName, serviceName, topicName string) {
	var isFound bool

	t.Lock()
	// check if topic is already in the queue
	for _, name := range t.inQueue[projectName+serviceName] {
		if name == topicName {
			isFound = true
		}
	}

	_, inCache := t.internal[projectName+serviceName][topicName]
	// the only topic that is not in the queue nor inside cache can be added to the queue
	if !isFound && !inCache {
		t.inQueue[projectName+serviceName] = append(t.inQueue[projectName+serviceName], topicName)
	}
	t.Unlock()
}

// DeleteFromQueueAndMarkMissing topic from the queue and marks it as missing
func (t *kafkaTopicCache) DeleteFromQueueAndMarkMissing(projectName, serviceName, topicName string) {
	t.Lock()
	for k, name := range t.inQueue[projectName+serviceName] {
		if name == topicName {
			t.inQueue[projectName+serviceName] = slices.Delete(t.inQueue[projectName+serviceName], k, k+1)
		}
	}

	t.missing[projectName+serviceName] = append(t.missing[projectName+serviceName], topicName)
	t.Unlock()
}

// GetMissing retrieves a list of missing topics
func (t *kafkaTopicCache) GetMissing(projectName, serviceName string) []string {
	t.RLock()
	defer t.RUnlock()

	return t.missing[projectName+serviceName]
}

// GetQueue retrieves a topics queue, retrieves up to 100 first elements
func (t *kafkaTopicCache) GetQueue(projectName, serviceName string) []string {
	t.RLock()
	defer t.RUnlock()

	if len(t.inQueue[projectName+serviceName]) >= 100 {
		return t.inQueue[projectName+serviceName][:99]
	}

	return t.inQueue[projectName+serviceName]
}

// FlushTopicCache for tests only!
func FlushTopicCache() {
	c := getTopicCache()
	c.Lock()
	for k := range c.internal {
		delete(c.internal, k)
		delete(c.inQueue, k)
		delete(c.missing, k)
		delete(c.v1list, k)
	}
	c.Unlock()
}

// SetV1List sets v1 topics list
func (t *kafkaTopicCache) SetV1List(projectName, serviceName string, list []*aiven.KafkaListTopic) {
	t.Lock()
	for _, v := range list {
		t.v1list[projectName+serviceName] = append(t.v1list[projectName+serviceName], v.TopicName)
	}
	t.Unlock()
}

// GetV1List retrieves a list of V1 kafka topic names
func (t *kafkaTopicCache) GetV1List(projectName, serviceName string) []string {
	t.RLock()
	defer t.RUnlock()

	return t.v1list[projectName+serviceName]
}
