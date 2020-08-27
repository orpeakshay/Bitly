package main

import (
  "fmt"
  "net/http"
  //"strings"
  "encoding/json"
  //"regexp"
  "github.com/streadway/amqp"
  "time"
)

//Global Variables

//Local Variable Set
//var shortIndex []string = []string{"12345","12346","12347","62345","13345","12349","12647","12335"}
//var urlMap map[string]string = make(map[string]string)
//var gatewayURL string = "localhost:8080"
//var cacheLimit int = 15
//var rabbitmqHost string = "localhost"
//var rabbitmqPort string = "5672"
//var rabbitmqUser string = "guest"
//var rabbitmqPassword string = "guest"
//var rabbitmqConnectionURL string = "amqp://"+rabbitmqUser+":"+rabbitmqPassword+"@"+rabbitmqHost+":"+rabbitmqPort+"/"
//var maxQueueLength int = 100
//var queueName string = "uniqueCodes-queue"

//AWS Variable Set
var shortIndex []string = []string{"12345","12346","12347","62345","13345","12349","12647","12335"}
var urlMap map[string]string = make(map[string]string)
var gatewayURL string = "3.226.139.223:8000/lr"
var cacheLimit int = 15
var rabbitmqHost string = "10.1.1.52"
var rabbitmqPort string = "5672"
var rabbitmqUser string = "test"
var rabbitmqPassword string = "test"
var rabbitmqConnectionURL string = "amqp://"+rabbitmqUser+":"+rabbitmqPassword+"@"+rabbitmqHost+":"+rabbitmqPort+"/"
var maxQueueLength int = 100
var queueName string = "uniqueCodes-queue"

func main(){
  fmt.Print("I am Control Panel.")
  fmt.Println("Creating root httpHandle at /")
  mux := http.NewServeMux()
  initRoutes(mux)
  //time.Sleep(10*time.Second)
  http.ListenAndServe(":8081", mux)
  for true {
    continue
  }
}

//Initialize All Routes
func initRoutes(mux *http.ServeMux){
  //Control Panel
  mux.HandleFunc("/", ping)
  mux.HandleFunc("/createShortLink", createShortLink)
}

//Route Functions
func ping(w http.ResponseWriter, r *http.Request){
  //w.Header().Set("Access-Control-Allow-Origin","*")
  var resultString string
  methods := []string{"GET"}
  _, found := Find(methods,r.Method)
  if !found {
    resultString = "Method not allowed on resource " + r.URL.Path
  } else if r.URL.Path == "/" {
    resultString = "Server is alive and accepting requests!"
  } else {
    resultString = "Invalid URL"
  }
  result := map[string]string{
      "accessed_url_path": r.URL.Path,
      "return_value": resultString,
    }
  //Prepare response
  setResponseHeaders(w)
  //w.Header().Set("Access-Control-Allow-Origin","*")
  json.NewEncoder(w).Encode(result)
}

func createShortLink(w http.ResponseWriter, r *http.Request){
  w.Header().Set("Access-Control-Allow-Origin","*")
  var resultString string
  var inputURL string
  methods := []string{"POST"}
  _, found := Find(methods,r.Method)
  if !found {
    resultString = "Method not allowed on resource " + r.URL.Path
  } else {
    //Extract request body
    var jsonData map[string]string
    json.NewDecoder(r.Body).Decode(&jsonData)
    inputURL = jsonData["url"]
    //Shorten URL using hash
    resultString = hashURL(inputURL)
  }
  //Send shortened URL
  result := map[string]string{
      "accessed_url_path": r.URL.Path,
      "return_value": "http://"+gatewayURL+"/"+resultString,
  }
  //Prepare response
  setResponseHeaders(w)
  json.NewEncoder(w).Encode(result)
  //dbConn := getConnectionToMySQL()
  //go storeToDatabase(dbConn, resultString, inputURL)
}

//Fetch code from message queue
func hashURL(fullURL string)(string) {
  var sha1_hash string
  //RabbitMQStart
  conn, err := getConnectionToRabbitMQ(rabbitmqConnectionURL)
  amqpChannel, err := conn.Channel();
  fmt.Print(err, "<-- amqpChannel || ")
  queueArgs := amqp.Table{"x-max-length":maxQueueLength}
  sha1_hash = string(consumeFromQueue(amqpChannel, "uniqueCodes-queue", queueArgs))
  
  queueArgs = amqp.Table{"x-max-length":maxQueueLength}
  body := "{ \"key\": \""+sha1_hash+"\", \"value\": \""+fullURL+"\"}"
  publishToQueue(amqpChannel, "shortenedURL-queue", body, queueArgs)
  
  if len(urlMap) >= cacheLimit {
    for k := range urlMap {
      //fmt.Println(k)
      delete(urlMap,k)
      break
    }
  }
  
  urlMap[sha1_hash] = fullURL
  amqpChannel.Close()
  fmt.Println(len(urlMap), "<-- Cache size")
  return sha1_hash
}

func getConnectionToRabbitMQ(connectionURL string)(*amqp.Connection, error){
  conn, err := amqp.Dial(rabbitmqConnectionURL)
  if err != nil {
    fmt.Println("Connection Failed")
  } else {
    fmt.Println("Connection to Rabbitmq was successful")
  }
  return conn,err
}

func consumeFromQueue(amqpChannel *amqp.Channel, queueName string, queueArgs amqp.Table)([]byte){
  var result []byte
	queue, err := amqpChannel.QueueDeclare(queueName, true, false, false, false, queueArgs)
	fmt.Print(err, "<-- queue || ")
	err = amqpChannel.Qos(1, 0, false)
	fmt.Print(err, "<-- err || ")
	messageChannel, err := amqpChannel.Consume(queue.Name, "", false, false, false, false, queueArgs,)
	fmt.Print(messageChannel, "<-- message channel || ")
  for d := range messageChannel {
	  fmt.Println("Received a message --> ", string(d.Body), " || Sending Acknowledgement")
    d.Ack(true)
    time.Sleep(1*time.Second)
    result = d.Body
    //amqpChannel.Close()
    break
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

func Find(slice []string, val string) (int, bool) {
  for i, item := range slice {
    if item == val {
      return i, true
    }
  }
  return -1, false
}

func setResponseHeaders(w http.ResponseWriter){
  w.Header().Set("Content-Type","application-json")
  //w.Header().Set("Access-Control-Allow-Origin","*")
  //w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
  //w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
  //w.Header().Set("Access-Control-Allow-Credentials","true")
}
