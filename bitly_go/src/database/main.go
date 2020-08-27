package main

import (
  "fmt"
  "time"
  "encoding/json"
  "github.com/streadway/amqp"
  "database/sql"
  _ "github.com/go-sql-driver/mysql"
)
//Global Variables

//Local Variables
//var rabbitmqHost string = "localhost"
//var rabbitmqPort string = "5672"
//var rabbitmqUser string = "guest"
//var rabbitmqPassword string = "guest"
//var rabbitmqConnectionURL string = "amqp://"+rabbitmqUser+":"+rabbitmqPassword+"@"+rabbitmqHost+":"+rabbitmqPort+"/"
//var maxQueueLength int = 100

//AWS Variables
var rabbitmqHost string = "10.1.1.52"
var rabbitmqPort string = "5672"
var rabbitmqUser string = "test"
var rabbitmqPassword string = "test"
var rabbitmqConnectionURL string = "amqp://"+rabbitmqUser+":"+rabbitmqPassword+"@"+rabbitmqHost+":"+rabbitmqPort+"/"
var maxQueueLength int = 100

func main(){
  //var sha1_hash string 
  fmt.Println("I am DB server")
  conn, _ := getConnectionToRabbitMQ(rabbitmqConnectionURL)
  //amqpChannel, err := conn.Channel();
  //fmt.Print(err, "<-- amqpChannel || ")
  //queueArgs := amqp.Table{"x-max-length":maxQueueLength}
  //consumeFromQueue(amqpChannel, "shortenedURL-queue", queueArgs))
  go addNewURLToDatabase("shortenedURL-queue", conn)
  go updateUsageStatsForURL("usedURL-queue", conn)
  for true {
    time.Sleep(10*time.Second)
  }
}

func addNewURLToDatabase(queueName string, conn *amqp.Connection){
  for true {
    //fmt.Println("Consuming from "+queueName)
    amqpChannel, err := conn.Channel();
    fmt.Print(err, "<-- amqpChannel || ")
    queueData := map[string]string{}
    queueArgs := amqp.Table{"x-max-length":maxQueueLength}
    jsonErr := json.Unmarshal(consumeFromQueue(amqpChannel, queueName, queueArgs), &queueData)
    amqpChannel.Close()
    if jsonErr != nil {
      fmt.Println(err)
    } else {
      //fmt.Println(queueData)
      dbConn := getConnectionToMySQL()
      storeToDatabase(dbConn, queueData["key"], queueData["value"]) //store this to database
    }
    time.Sleep(1*time.Second)
  }
}

func updateUsageStatsForURL(queueName string, conn *amqp.Connection){
  for true {
    amqpChannel, err := conn.Channel();
    fmt.Print(err, "<-- amqpChannel || ")
    queueData := map[string]string{}
    queueArgs := amqp.Table{"x-max-length":maxQueueLength}
    jsonErr := json.Unmarshal(consumeFromQueue(amqpChannel, queueName, queueArgs), &queueData)
    amqpChannel.Close()
    if jsonErr != nil {
      fmt.Println(err)
    } else {
      fmt.Println(queueData)
      dbConn := getConnectionToMySQL()
      dbUpdateQuery := "update shortcodetolongurl set access_count=access_count+1 where code='"+queueData["key"]+"'" 
      dbInsertQuery := "insert into access_stats(code, access_time) values('"+queueData["key"]+"', current_timestamp)"
      update, err := dbConn.Query(dbUpdateQuery) 
      if err!= nil {
        fmt.Println("Failed to update record", err)
      } else {
        fmt.Println("Updated record successfully", update)
      }
      insert, insertErr := dbConn.Query(dbInsertQuery) 
      if insertErr!= nil {
        fmt.Println("Failed to insert record", insertErr)
      } else {
        fmt.Println("Inserted record successfully", insert)
      }
      //storeToDatabase(dbConn, queueData["key"], queueData["value"]) //store this to database
      time.Sleep(1*time.Second)
    }
  }
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

func getConnectionToRabbitMQ(connectionURL string)(*amqp.Connection, error){
  conn, err := amqp.Dial(rabbitmqConnectionURL)
  if err != nil {
    fmt.Println("Connection Failed")
  } else {
    fmt.Println("Connection to Rabbitmq was successful")
  }
  return conn,err
}

func getConnectionToMySQL()(*sql.DB){
  mysql_user := "localhost"
  mysql_password := "localhost@123"
  mysql_host := "localhost"
  mysql_port := "3306"
  mysql_db := "bitly"
  //fmt.Println("Connecting to MySQL Database")
  db, err := sql.Open("mysql",mysql_user+":"+mysql_password+"@tcp("+mysql_host+":"+mysql_port+")/"+mysql_db)
  if err != nil {
    fmt.Println("Connection to MySQL failed")
  } else {
    fmt.Println("Connection to MySQL was successful @ "+mysql_host+":"+mysql_port)
  }
  return db
}

func storeToDatabase(dbConn *sql.DB, code, inputURL string){
  fmt.Println("Storing "+code+"/"+inputURL+" to database . . .")
  dbQuery := "insert into shortcodetolongurl(code,longurl) values('"+code+"','"+inputURL+"')"
  //var sampleURL string
  insert, err := dbConn.Query(dbQuery) //.Scan(&sampleURL) //querying a single row
  if err!= nil {
      fmt.Println("Insert failed : ",err)
  } else {
      insert.Close()
  }
  //time.Sleep(10*time.Second)
  fmt.Println("Successfully written to database")
}