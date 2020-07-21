package service

import (
	"github.com/mxlxm/RocketmqExporter/constant"
	"github.com/mxlxm/RocketmqExporter/model"
	"github.com/mxlxm/RocketmqExporter/utils"
	"github.com/mxlxm/RocketmqExporter/wrapper"
	"strings"

	cmap "github.com/orcaman/concurrent-map"
	"sync"
)

func MsgUnconsumedCount(rocketmqConsoleIPAndPort string) *model.MsgDiff {

	//获取rocketmq集群中的topicNameList
	topicNameArray := wrapper.GetTopicNameList(rocketmqConsoleIPAndPort)
	if topicNameArray == nil {
		return nil
	}

	//获取不纳入监控的topicNameList
	var ignoredTopicNameList = constant.GetIgnoredTopicArray()

	var rt *model.MsgDiff = new(model.MsgDiff)

	var diff_Detail_Slice []*model.MsgDiff_Detail

	//按照topic聚合msgDiff
	//var diff_Topic_Slice []model.MsgDiff_Topic = []model.MsgDiff_Topic{}
	var diff_Topic_Map = make(map[string]*model.MsgDiff_Topic)
	var diff_Topic_cMap = cmap.New()

	//按照consumerGroup聚合msgDiff
	//var diff_ConsumerGroup_Slice []model.MsgDiff_ConsumerGroup = []model.MsgDiff_ConsumerGroup{}
	var diff_ConsumerGroup_Map = make(map[string]*model.MsgDiff_ConsumerGroup)
	var diff_ConsumerGroup_cMap = cmap.New()

	//按照topic, consumeGroup聚合msgDiff
	//var diff_Topic_ConsumerGroup_Slice []model.MsgDiff_Topics_ConsumerGroup = []model.MsgDiff_Topics_ConsumerGroup{}
	var diff_Topic_ConsumerGroup_Map = make(map[string]*model.MsgDiff_Topic_ConsumerGroup)
	var diff_Topic_ConsumerGroup_cMap = cmap.New()

	//按照broker聚合msgDiff
	//var diff_Broker_Slice []model.MsgDiff_Broker = []model.MsgDiff_Broker{}
	var diff_Broker_Map = make(map[string]*model.MsgDiff_Broker)
	var diff_Broker_cMap = cmap.New()

	//按照clientInfo聚合msgDiff
	//var diff_Clientinfo_Slice []model.MsgDiff_ClientInfo = []model.MsgDiff_ClientInfo{}
	var diff_Clientinfo_Map = make(map[string]*model.MsgDiff_ClientInfo)
	var diff_Clientinfo_cMap = cmap.New()

	//按照queue聚合msgDiff
	//var MsgDiff_Queue_Slice []model.MsgDiff_Queue = []model.MsgDiff_Queue{}
	var diff_Queue_Map = make(map[string]*model.MsgDiff_Queue)
	var diff_Queue_cMap = cmap.New()

	var wg sync.WaitGroup
	var parallel = make(chan bool, 10)

	for _, topicName := range topicNameArray {
		index := utils.Contains(ignoredTopicNameList, topicName)
		if index >= 0 {
			continue
		}

		wg.Add(1)
		parallel <- true
		go func(topicName string) {
			defer func() {
				wg.Done()
				<-parallel
			}()

			var data *model.ConsumerList_By_Topic = wrapper.GetConsumerListByTopic(rocketmqConsoleIPAndPort, topicName)

			if data == nil {
				return
			}

			topicConsumerGroups := data.Data

			for cgName, consumerInfo := range topicConsumerGroups {
				topic := consumerInfo.Topic
				//diffTotal := consumerInfo.DiffTotal
				//lastTimestamp := consumerInfo.LastTimestamp

				//获取当前consumer信息及对应的rocketmq-queue的信息
				queueStatInfoList := consumerInfo.QueueStatInfoList

				for _, queue := range queueStatInfoList {

					var diffDetail *model.MsgDiff_Detail = new(model.MsgDiff_Detail)

					brokerName := queue.BrokerName
					queueId := queue.QueueId

					clientInfo := queue.ClientInfo
					consumerClientIP := ""
					consumerClientPID := ""
					if &clientInfo != nil {
						temp_array := strings.Split(clientInfo, "@")
						if temp_array != nil {
							if len(temp_array) == 1 {
								consumerClientIP = temp_array[0]
							} else if len(temp_array) == 2 {
								consumerClientIP = temp_array[0]
								consumerClientPID = temp_array[1]
							}
						}
					}

					diff := int(queue.BrokerOffset) - int(queue.ConsumerOffset)
					//lastTimestamp = queue.LastTimestamp

					diffDetail.Broker = brokerName
					diffDetail.QueueId = queueId
					diffDetail.ConsumerClientIP = consumerClientIP
					diffDetail.ConsumerClientPID = consumerClientPID
					diffDetail.Diff = diff
					diffDetail.Topic = topic
					diffDetail.ConsumerGroup = cgName
					diff_Detail_Slice = append(diff_Detail_Slice, diffDetail)

					//按照topic进行msgDiff聚合
					if v, ok := diff_Topic_cMap.Get(topic); ok {
						//如果已经存在，计算diff
						//diff_Topic_Map[topic].Diff = diff_Topic_Map[topic].Diff + diff
						tmp := v.(*model.MsgDiff_Topic)
						tmp.Diff += diff
						diff_Topic_cMap.Set(topic, tmp)
					} else {
						var diffTopic *model.MsgDiff_Topic = new(model.MsgDiff_Topic)

						diffTopic.Diff = diff
						diffTopic.Topic = topic

						// diff_Topic_Map[topic] = diffTopic
						diff_Topic_cMap.Set(topic, diffTopic)
					}

					//按照consumerGroup进行msgDiff聚合
					if v, ok := diff_ConsumerGroup_cMap.Get(cgName); ok {
						tmp := v.(*model.MsgDiff_ConsumerGroup)
						tmp.Diff += diff
						diff_ConsumerGroup_cMap.Set(cgName, tmp)
						// diff_ConsumerGroup_Map[cgName].Diff = diff_ConsumerGroup_Map[cgName].Diff + diff
					} else {
						var diffConsumerGroup *model.MsgDiff_ConsumerGroup = new(model.MsgDiff_ConsumerGroup)

						diffConsumerGroup.ConsumerGroup = cgName
						diffConsumerGroup.Diff = diff

						//diff_ConsumerGroup_Map[cgName] = diffConsumerGroup
						diff_ConsumerGroup_cMap.Set(cgName, diffConsumerGroup)
					}

					//按照topic, consumerGroup进行msgDiff聚合
					topic_cgName := topic + ":" + cgName
					if v, ok := diff_Topic_ConsumerGroup_cMap.Get(topic_cgName); ok {
						tmp := v.(*model.MsgDiff_Topic_ConsumerGroup)
						tmp.Diff += diff
						diff_Topic_ConsumerGroup_cMap.Set(topic_cgName, tmp)
						// diff_Topic_ConsumerGroup_Map[topic_cgName].Diff = diff_Topic_ConsumerGroup_Map[topic_cgName].Diff + diff

					} else {
						var diff_topic_cg *model.MsgDiff_Topic_ConsumerGroup = new(model.MsgDiff_Topic_ConsumerGroup)

						diff_topic_cg.ConsumerGroup = cgName
						diff_topic_cg.Diff = diff
						diff_topic_cg.Topic = topic

						// diff_Topic_ConsumerGroup_Map[topic_cgName] = diff_topic_cg
						diff_Topic_ConsumerGroup_cMap.Set(topic_cgName, diff_topic_cg)

					}

					//按照broker进行msgDiff聚合
					if v, ok := diff_Broker_cMap.Get(brokerName); ok {
						tmp := v.(*model.MsgDiff_Broker)
						tmp.Diff += diff
						diff_Broker_cMap.Set(brokerName, tmp)
						//diff_Broker_Map[brokerName].Diff = diff_Broker_Map[brokerName].Diff + diff
					} else {
						var diff_Broker *model.MsgDiff_Broker = new(model.MsgDiff_Broker)

						diff_Broker.Broker = brokerName
						diff_Broker.Diff = diff

						//diff_Broker_Map[brokerName] = diff_Broker
						diff_Broker_cMap.Set(brokerName, diff_Broker)
					}

					//按照queueId进行msgDiff聚合
					queuestr := brokerName + ":" + string(queueId)
					if v, ok := diff_Queue_cMap.Get(queuestr); ok {
						tmp := v.(*model.MsgDiff_Queue)
						tmp.Diff += diff
						diff_Queue_cMap.Set(queuestr, tmp)
						// diff_Queue_Map[queuestr].Diff = diff_Queue_Map[queuestr].Diff + diff
					} else {
						var diff_Queue *model.MsgDiff_Queue = new(model.MsgDiff_Queue)

						diff_Queue.Broker = brokerName
						diff_Queue.Diff = diff
						diff_Queue.QueueId = queueId

						//	diff_Queue_Map[queuestr] = diff_Queue
						diff_Queue_cMap.Set(queuestr, diff_Queue)
					}

					//按照clientInfo进行msgDiff聚合

					if v, ok := diff_Clientinfo_cMap.Get(clientInfo); ok {
						tmp := v.(*model.MsgDiff_ClientInfo)
						tmp.Diff += diff
						diff_Clientinfo_cMap.Set(clientInfo, tmp)
						//diff_Clientinfo_Map[clientInfo].Diff = diff_Clientinfo_Map[clientInfo].Diff + diff
					} else {
						var diff_ClientInfo *model.MsgDiff_ClientInfo = new(model.MsgDiff_ClientInfo)

						diff_ClientInfo.ConsumerClientIP = consumerClientIP
						diff_ClientInfo.ConsumerClientPID = consumerClientPID
						diff_ClientInfo.Diff = diff

						//diff_Clientinfo_Map[clientInfo] = diff_ClientInfo
						diff_Clientinfo_cMap.Set(clientInfo, diff_ClientInfo)
					}

				}
			}

		}(topicName)
	}
	wg.Wait()

	rt.MsgDiff_Details = diff_Detail_Slice

	for k, v := range diff_Topic_cMap.Items() {
		diff_Topic_Map[k] = v.(*model.MsgDiff_Topic)
	}
	rt.MsgDiff_Topics = diff_Topic_Map

	for k, v := range diff_ConsumerGroup_cMap.Items() {
		diff_ConsumerGroup_Map[k] = v.(*model.MsgDiff_ConsumerGroup)
	}
	rt.MsgDiff_ConsumerGroups = diff_ConsumerGroup_Map

	for k, v := range diff_Topic_ConsumerGroup_cMap.Items() {
		diff_Topic_ConsumerGroup_Map[k] = v.(*model.MsgDiff_Topic_ConsumerGroup)
	}
	rt.MsgDiff_Topics_ConsumerGroups = diff_Topic_ConsumerGroup_Map

	for k, v := range diff_Broker_cMap.Items() {
		diff_Broker_Map[k] = v.(*model.MsgDiff_Broker)
	}
	rt.MsgDiff_Brokers = diff_Broker_Map

	for k, v := range diff_Queue_cMap.Items() {
		diff_Queue_Map[k] = v.(*model.MsgDiff_Queue)
	}
	rt.MsgDiff_Queues = diff_Queue_Map

	for k, v := range diff_Clientinfo_cMap.Items() {
		diff_Clientinfo_Map[k] = v.(*model.MsgDiff_ClientInfo)
	}
	rt.MsgDiff_ClientInfos = diff_Clientinfo_Map

	return rt

}
