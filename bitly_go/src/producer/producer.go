package main

import (
  "fmt"
  "time"
  "bufio"
  "os"
  // "io/ioutil"
  "math/rand"
  "strconv"
  "github.com/streadway/amqp"
)

func main(){
  var rabbitmqUser = "guest"
  var rabbitmqPassword = "guest"
  var rabbitmqHost = "localhost"
  var rabbitmqPort = "5672"
  var rabbitmqConnectionURL string = "amqp://"+rabbitmqUser+":"+rabbitmqPassword+"@"+rabbitmqHost+":"+rabbitmqPort+"/"
  var maxQueueLength int = 100
  var queueName string = "uniqueCodes-queue"
  conn, err := amqp.Dial(rabbitmqConnectionURL)
  if err != nil {
      fmt.Println("Connection Failed")
  } else {
      fmt.Println("Connection to Rabbitmq was successful")
  }
  for true {
    uniqueCode := getUniqueCode()
    //fmt.Println(uniqueCode)
    fmt.Print("queue <-- ",uniqueCode, " || ")
    time.Sleep(3*time.Second)
  	amqpChannel, err := conn.Channel()
	fmt.Print(err, "<-- amqpChannel || ")
    queueArgs := amqp.Table{"x-max-length":maxQueueLength,"x-overflow":"reject-publish"}
    publishToQueue(amqpChannel, "uniqueCodes-queue", uniqueCode, queueArgs)
    queueDetails, _ := amqpChannel.QueueInspect(queueName)
    fmt.Println(queueDetails.Messages, " <-- queue cap")
    for queueDetails.Messages >= maxQueueLength {
      time.Sleep(2*time.Second)
      queueDetails, _ = amqpChannel.QueueInspect(queueName)
      continue
    }
  }

}

func getUniqueCode()(string){
 basePath := "/home/ubuntu/cmpe281-akshay-sjsu-173/bitly/bitly_go/bin/url_hash_codes/a-z_6/hash"
 //fmt.Println("Trying to get a unique code")
 randIndex := rand.Intn(20) //%len(shortIndex)
 filePath := basePath + padLeft(randIndex, 2, "0")
 return getFileContents(filePath)
 //return "someCode273"
}

func padLeft(num, length int, pad string)(string){
  numStr := strconv.Itoa(num)
  if len(numStr) == length {
    return numStr
  }
  for i:=1; i<=length-1; i++ {
    numStr = pad+numStr
  }
  return numStr
}

func getFileContents(filePath string)(string){
  fmt.Print(filePath, " <-- filename || ")
  //data, err := ioutil.ReadFile(filePath)
  var result string
  var randIndex int
  data, err := os.Open(filePath)
  if err != nil {
    fmt.Println(err)
  } else {
    //print(string(data),len(string(data)))
    scanner := bufio.NewScanner(data)
    scanner.Split(bufio.ScanLines)
    i := 0
    var dataLines []string
    for scanner.Scan() {
      i += 1
      dataLines = append(dataLines, scanner.Text())
    }
    randIndex = rand.Intn(len(dataLines))
    result = dataLines[randIndex]
    //replace selected element with last element. Assign "" to last element
    dataLines[randIndex] = dataLines[len(dataLines)-1]
    dataLines[len(dataLines)-1] = ""
    dataLines = dataLines[:len(dataLines)-1]
    //dataWrite, _ := os.OpenFile("test.txt",os.O_WRONLY,0644)
    //dataWriter := bufio.NewWriter(dataWrite)
    dataWrite, _ := os.Create(filePath)
    //sampleStringArray := []string{"a","b","c","d",}
    for _,data := range dataLines{
      //fmt.Println("Writing to text.txt")
      _,_ = dataWrite.WriteString(data+"\n")
    }
    //fmt.Println(randIndex, result)
    data.Close()
  }
  return result
}

func publishToQueue(amqpChannel *amqp.Channel, queueName, body string, queueArgs amqp.Table){
    queue, err := amqpChannel.QueueDeclare(queueName, true, false, false, false, queueArgs)
    fmt.Print(err, "<-- queue, ")
	err = amqpChannel.Publish("", queue.Name, false, false, amqp.Publishing{
        //Headers: amqp.Table{"Channel.NotifyReturn":true,"Channel.NotifyPublish":true},
        DeliveryMode: amqp.Persistent,
        ContentType:  "application/json",
        Body: []byte(body),
	})
    //amqpChannel.Close()
}
