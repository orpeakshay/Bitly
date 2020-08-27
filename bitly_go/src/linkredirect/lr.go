package main

import (
  "fmt"
  "io/ioutil"
  "net/http"
  "strings"
  "bytes"
  "regexp"
  "encoding/json"
  "database/sql"
  "github.com/streadway/amqp"
  _ "github.com/go-sql-driver/mysql"
)

//Global Variables

//Local Variable Set
//var rabbitmqHost string = "localhost"
//var rabbitmqPort string = "5672"
//var rabbitmqUser string = "guest"
//var rabbitmqPassword string = "guest"
//var rabbitmqConnectionURL string = "amqp://"+rabbitmqUser+":"+rabbitmqPassword+"@"+rabbitmqHost+":"+rabbitmqPort+"/"
//var maxQueueLength int = 100
//var queueName string = "uniqueCodes-queue"
//var cache_server string = "localhost:9001"
////var cache_port string = "9001"
//var cacheLimit int = 3
//var mysql_host string = "localhost"
//var gatewayURL string = "localhost:8080"

//AWS variable Set
var rabbitmqHost string = "10.1.1.52"
var rabbitmqPort string = "5672"
var rabbitmqUser string = "test"
var rabbitmqPassword string = "test"
var rabbitmqConnectionURL string = "amqp://"+rabbitmqUser+":"+rabbitmqPassword+"@"+rabbitmqHost+":"+rabbitmqPort+"/"
var maxQueueLength int = 100
var queueName string = "uniqueCodes-queue"
var cache_server string = "internal-bitly-nosql-1883624996.us-east-1.elb.amazonaws.com"
//var cache_port string = "9090"
var cacheLimit int = 10
var mysql_host string = "10.1.1.42"
var gatewayURL string = "3.226.139.223:8000/lr"

func main(){
  fmt.Println("I am LR server")
  fmt.Println("Creating root httpHandle at /")
  mux := http.NewServeMux()
  initRoutes(mux)
  //time.Sleep(10*time.Second)
  http.ListenAndServe(":8080", mux)
}

//Initialize All Routes
func initRoutes(mux *http.ServeMux){

  //Link Redirect
  mux.HandleFunc("/", ping)
  mux.HandleFunc("/trendingLinks",getTrendingLinks)
  //mux.HandleFunc("/createShortLink", createShortLink)
  //Link Redirect
  //mux.HandleFunc("/stats", getStats)
}

//Route Functions
func ping(w http.ResponseWriter, r *http.Request){
  //w.Header().Set("Access-Control-Allow-Origin","*")
  var resultString string
  methods := []string{"GET"}
  _, found := Find(methods,r.Method)
  if !found {
    resultString = "Method not allowed on resource " + r.URL.Path
  } else {
    //Check server status
    reg := regexp.MustCompile(`^[\w]{6}$`) //assuming that shortlink hash will be of length 6
    if len(reg.FindAllString(strings.TrimPrefix(r.URL.Path, "/"), -1)) > 0 {
	    resultString = redirectToFullLink(w, r)
      if resultString != "" {
        w.Header().Set("Location",resultString)
        http.Redirect(w, r, resultString, 302) //Status 302 indicates the browser to redirect to resultString
      } else {
        resultString = "Invalid URL"
      }
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

func redirectToFullLink(w http.ResponseWriter, r *http.Request)(string){

  var resultString string
  var fullURL string
  methods := []string{"GET"}
  _, found := Find(methods,r.Method)
  if !found {
    resultString = "Method not allowed on resource " + r.URL.Path
  } else {
    _, fullURL = getDataFromCache(strings.TrimPrefix(r.URL.Path, "/"))
    fmt.Println(fullURL," <-- fullURL")
    //fullURL := urlMap[strings.TrimPrefix(r.URL.Path, "/")] //fetch from cache, if unavailable, fetch from database
    if fullURL != "" {
      resultString = fullURL
      //"Redirecting to fully qualified link : "+string(fullURL)
    } else {
      fmt.Println("Cache miss. Fetching from Database.")
      dbConn := getConnectionToMySQL()
      dbSelectQuery := "select longurl from shortcodetolongurl where code='"+strings.TrimPrefix(r.URL.Path, "/")+"'"
      var sampleURL string
      dbConn.QueryRow(dbSelectQuery).Scan(&sampleURL) //querying a single row
      resultString = sampleURL
    }
  }
  
  //make entry to cache
  //cacheSize, key := getCacheSize()
  if resultString != "" {
    statusCode := writeDataToCache(strings.TrimPrefix(r.URL.Path, "/"),resultString)
    if statusCode == 400 {
        fmt.Println("update cache", statusCode)
        updateCache(strings.TrimPrefix(r.URL.Path, "/"), fullURL)
    } else if statusCode == 200 {
        fmt.Println("Succesfully written to cache", statusCode)
    }
  }
      
  //Send used URL to the queue
  if resultString != "" {
    //add to strings.TrimPrefix(r.URL.Path to the queue
    conn, err := getConnectionToRabbitMQ(rabbitmqConnectionURL)
    amqpChannel, err := conn.Channel();
    fmt.Print(err, "<-- amqpChannel || ")
    queueArgs := amqp.Table{"x-max-length":maxQueueLength}
    body := "{ \"key\": \""+strings.TrimPrefix(r.URL.Path,"/")+"\"}"
    publishToQueue(amqpChannel, "usedURL-queue", body, queueArgs)
    amqpChannel.Close()
    conn.Close()
  }
  fmt.Print(resultString, "<-- redirecting to")
  return resultString
}

func getTrendingLinks(w http.ResponseWriter, r *http.Request){
   var copyOfTrendingLinks = make(map[string]map[string]string)
   var resultString string
   methods := []string{"GET"}
   _, found := Find(methods,r.Method)
   if !found {
     resultString = "Method not allowed on resource " + r.URL.Path
   } else {
     dbConn := getConnectionToMySQL()
     dbSelectQuery := "select code,access_count from shortcodetolongurl order by access_count desc limit 10"
     queryData, err := dbConn.Query(dbSelectQuery)
     if err != nil {
      fmt.Println(err)
     } else {
      for queryData.Next() {
        var code string
        var access_count string
        _ = queryData.Scan(&code,&access_count)
        m := getPeriodStats(code,"minute",dbConn)
        h := getPeriodStats(code,"hour",dbConn)
        d := getPeriodStats(code,"day",dbConn)
        copyOfTrendingLinks["http://"+gatewayURL+"/"+code] = map[string]string{
                "last-1-minute": m,
                "last-1-hour": h,
                "last-1-day": d,
                "all-time": access_count,
            }
      }
     }
   }
   
   fmt.Println(resultString)
   //Prepare Response
   result := map[string]interface{}{
      "accessed_url_path": r.URL.Path,
      "return_value": copyOfTrendingLinks,
  }
  //Prepare response
  setResponseHeaders(w)
  json.NewEncoder(w).Encode(result)
}

func getPeriodStats(shortCode, period string, dbConn *sql.DB)(string){
    dbSelectQuery := "select count(code) from access_stats where code='"+shortCode+"' and access_time > date_sub(now(), interval 1 "+period+")"
    queryData, err := dbConn.Query(dbSelectQuery)
    var code string
     if err != nil {
      fmt.Println(err)
     } else {
      for queryData.Next() {
        _ = queryData.Scan(&code)
        //listOfTrendingLinks = append(listOfTrendingLinks, "http://"+gatewayURL+"/"+code)
      }
     }
     return code
}

func getDataFromCache(shortCode string)(int,string){
    result := map[string]string{}
    //var resultURL string
    resp, err := http.Get("http://"+cache_server+"/api/"+shortCode)
    if err != nil {
      fmt.Println(err, " <-- Received an error")
    } else {
      body, err := ioutil.ReadAll(resp.Body)
      _ = json.Unmarshal(body, &result)
      fmt.Println(result, err)
      //for item := range(result) {
      //  if result[item]["key"] == shortCode {
      //    resultURL = result[item]
      //    break
      //  }
      //}
    }
    return resp.StatusCode,result["longurl"]
}

func writeDataToCache(shortCode, longURL string)(int){
  reqBody, _ := json.Marshal(map[string]string{
      "longurl": longURL,
    })
  resp, err := http.Post("http://"+cache_server+"/api/"+shortCode,"application/json",bytes.NewBuffer(reqBody))
  if err != nil || resp.StatusCode != 200 {
    fmt.Println("Could not create key")
  } else {
    fmt.Println(resp.StatusCode)
  }
  return resp.StatusCode
}

func getCacheSize()(int, string){
  result := []map[string]interface{}{}
  topKey := ""
  resp, err := http.Get("http://"+cache_server+"/api")
  if err != nil {
      fmt.Println(err, " <-- Received an error")
  } else {
    body, _ := ioutil.ReadAll(resp.Body)
    _ = json.Unmarshal(body, &result)
    //fmt.Println(result[0]["key"], err)
  }
  if len(result) > 0 {
    topKey = result[0]["key"].(string)
  }
  return len(result), topKey
}

func updateCache(shortCode, fullURL string){
  httpClient := &http.Client{}
  reqPutBody, _ := json.Marshal(map[string]string{
     "longurl": fullURL,
    })
  reqPut, _ := http.NewRequest(http.MethodPut, "http://"+cache_server+"/api/"+shortCode, bytes.NewBuffer(reqPutBody))
  reqPut.Header.Set("Content-Type","application/json; charset=utf-8")
  respPut, errPut := httpClient.Do(reqPut)
  fmt.Println(respPut.StatusCode, errPut)
}

func deleteFromCache(shortCode string){
  httpClient := &http.Client{}
  reqDelete, _ := http.NewRequest(http.MethodDelete, "http://"+cache_server+"/api/"+shortCode, nil)
  reqDelete.Header.Set("Content-Type","application/json; charset=utf-8")
  respDelete, errDelete := httpClient.Do(reqDelete)
  fmt.Println(respDelete.StatusCode, errDelete)
}

func getConnectionToMySQL()(*sql.DB){
  mysql_user := "localhost"
  mysql_password := "localhost@123"
  mysql_port := "3306"
  mysql_db := "bitly"
  //fmt.Println("Connecting to MySQL Database")
  db, err := sql.Open("mysql",mysql_user+":"+mysql_password+"@tcp("+mysql_host+":"+mysql_port+")/"+mysql_db)
  if err != nil {
    fmt.Println("Connection to MySQL failed",err)
  } else {
    fmt.Println("Connection to MySQL was successful @ "+mysql_host+":"+mysql_port)
  }
  return db
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
}