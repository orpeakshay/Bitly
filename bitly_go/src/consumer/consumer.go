package main

import (
  "fmt"
  "time"
  _ "os/exec"
  "strings"
  _ "bytes"
  "net/http"
  "encoding/json"
  _ "math/rand"
  "regexp"
  _ "crypto/sha1" //to generate hash
  _"encoding/base64" //to get a string representation of the hash
  //rabbitmq
  "github.com/streadway/amqp"
  //mysql
  "database/sql"
   _ "github.com/go-sql-driver/mysql"
  //"github.com/aws/aws-sdk-go/service/sqs"
)

//Global Variables
var shortIndex []string = []string{"12345","12346","12347","62345","13345","12349","12647","12335"}
var urlMap map[string]string = make(map[string]string)
var rabbitmqConnectionURL string = "amqp://guest:guest@localhost:5672/"
var maxQueueLength int = 100
var queueName string = "uniqueCodes-queue"

func main() {
  fmt.Println("Creating root httpHandle at /")
  mux := http.NewServeMux()
  initRoutes(mux)
  //time.Sleep(10*time.Second)
  http.ListenAndServe(":8080", mux)
}
  
//Initialize All Routes
func initRoutes(mux *http.ServeMux){
  
  //Control Paneli
  mux.HandleFunc("/", ping)
  mux.HandleFunc("/createShortLink", createShortLink)
  
  //Link Redirect
  mux.HandleFunc("/stats", getStats)
  
}

//Route Functions
func ping(w http.ResponseWriter, r *http.Request){
  
  var resultString string
  methods := []string{"GET"}
  _, found := Find(methods,r.Method)
  if !found {
    resultString = "Method not allowed on resource " + r.URL.Path
  } else {
    //Check server status
    reg := regexp.MustCompile(`[\w]{6}`) //assuming that shortlink hash will be of length 6
    //fmt.Println(reg.FindAllString(r.URL.Path, -1))
    if len(reg.FindAllString(r.URL.Path, -1)) > 0 {
	  resultString = redirectToFullLink(w, r)
      w.Header().Set("Location",resultString)
      http.Redirect(w, r, resultString, 302) //Status 302 indicates the browser to redirect to resultString
    } else if r.URL.Path == "/" {
        resultString = "Server is alive and accepting requests!"
    } else {
        resultString = "Invalid URL"
    }
  }
  result := map[string]string{
      "accessed_url_path": r.URL.Path,
      "return_value": resultString,
    }
  //Prepare response
  setResponseHeaders(w)
  json.NewEncoder(w).Encode(result)
  
}

func getStats(w http.ResponseWriter, r *http.Request){
  
  var resultString string
  methods := []string{"GET"}
  _, found := Find(methods,r.Method)
  if !found {
    resultString = "Method not allowed on resource " + r.URL.Path
  } else {
    //Fetch trend stats from the server
    resultString = "Trying to fetch some stats."
  }
  //Send trend stats
  result := map[string]string{
      "accessed_url_path": r.URL.Path,
      "return_value": resultString,
    }
  //Prepare response
  setResponseHeaders(w)
  json.NewEncoder(w).Encode(result) 
}

func createShortLink(w http.ResponseWriter, r *http.Request){
  
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
      "return_value": r.Host+"/"+resultString,
  }
  //Prepare response
  setResponseHeaders(w)
  json.NewEncoder(w).Encode(result)
  dbConn := getConnectionToMySQL()
  go storeToDatabase(dbConn, resultString, inputURL)
}

func redirectToFullLink(w http.ResponseWriter, r *http.Request)(string){
  
  var resultString string
  methods := []string{"GET"}
  _, found := Find(methods,r.Method)
  if !found {
    resultString = "Method not allowed on resource " + r.URL.Path
  } else {
    fullURL := urlMap[strings.TrimPrefix(r.URL.Path, "/")] //fetch from cache, if unavailable, fetch from database
    //res := map[string]interface{}{}
    //_ = json.Unmarshal(consumeFromQueue(amqpChannel, "shortenedURL-queue", queueArgs), &res)
    //fmt.Println(res)
    dbConn := getConnectionToMySQL()
    dbSelectQuery := "select longurl from shortcodetolongurl where code='"+strings.TrimPrefix(r.URL.Path, "/")+"'"
    dbUpdateQuery := "update shortcodetolongurl set access_count=access_count+1 where code='"+strings.TrimPrefix(r.URL.Path, "/")+"'"
    var sampleURL string
    go func(dbUpdateQuery string){
        update, err := dbConn.Query(dbUpdateQuery)
        if err != nil {
            fmt.Println("Update failed : ", err)
        } else {
            fmt.Println("Updated db record")
            update.Close()
        }
    }(dbUpdateQuery)
    if fullURL != "" {
        resultString = fullURL 
        //"Redirecting to fully qualified link : "+string(fullURL)
    } else {
        dbConn.QueryRow(dbSelectQuery).Scan(&sampleURL) //querying a single row
        resultString = sampleURL
    }
  }
  //Send shortened URL
  return resultString
}

//Common Functions
func setResponseHeaders(w http.ResponseWriter){
  w.Header().Set("Content-Type","application-json")
}

func Find(slice []string, val string) (int, bool) {
    for i, item := range slice {
        if item == val {
            return i, true
        }
    }
    return -1, false
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
    
    if len(urlMap) >= 15 {
        for k := range urlMap {
            //fmt.Println(k)
            delete(urlMap,k)
            break
        }
    }
    urlMap[sha1_hash] = fullURL
    amqpChannel.Close()
    fmt.Println(urlMap)
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

func getConnectionToMySQL()(*sql.DB){
  mysql_user := "localhost"
  mysql_password := "localhost@123"
  mysql_host := "localhost"
  mysql_port := "3306"
  mysql_db := "bitly"
  //searchCode := "achlot"
  //fmt.Println("Connecting to MySQL Database")
  db, err := sql.Open("mysql",mysql_user+":"+mysql_password+"@tcp("+mysql_host+":"+mysql_port+")/"+mysql_db)
  if err != nil {
    fmt.Println("Connection to MySQL failed")
  } else {
    fmt.Println("Connection to MySQL was successful")
  }
  return db
}


func storeToDatabase(dbConn *sql.DB, code, inputURL string){
    fmt.Println("Storing "+code+"/"+inputURL+" to database . . .")
    dbQuery := "insert into shortcodetolongurl(code,longurl) values('"+code+"','"+inputURL+"')"
    //var sampleURL string
    insert, err := dbConn.Query(dbQuery) //.Scan(&sampleURL) //querying a single row
    if err!= nil {
        fmt.Println("Inser failed : ",err)
    } else {
        insert.Close()
    }
    //time.Sleep(10*time.Second)
    fmt.Println("Successfully written to database")
}