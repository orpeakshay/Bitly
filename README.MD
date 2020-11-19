# Bitly Project
#

<b>1. Getting Started :</b>
   The requirements were simple. Create a service s close to Bitly as possible. After some self-assessment I decided to follow these steps:
   -  Learn to code in Golang
   -  Decide the algorithm to be used for creating short codes
   -  Read up on CQRS and Event Sourcing
   -  Understand how message queues work
   -  Figure out which message queue is best suited for the use case
   -  Understand how Kong API routes the requests
   -  Finalize the infrastructure design
   -  Deploy infrastructure on AWS/GCP
   
<b>2. Deployment Diagram :</b>
   -  The most important part of the project was drawing a diagram. Having the "big picture" in mind helped give direction to code as
      well as all other tasks that I was doing simultaneously. The following diagram represents my implementation of the system in
      AWS. The deployment in GCP was similar with slight differences.
   ![Bitly Deployment Diagram](https://github.com/orpeakshay/Bitly/blob/master/Bitly%20-%20Deployment%20Diagram.png)
  
<b>3. Design Decisions :</b>
   -  Golang -
      -  I wanted to understand how a web server running on Golang functions. Hence, I made a decision to use the native "http/mux" instead of using 3rd party libraries like "gorilla/mux"
      -  I experimented by using channels as well as goroutines to implement parallel functions and finally decided that goroutines work fine for my use case. Goroutines simply execute in the background which means I did not need to keep track of channel size and other parameters while coding my APIs
   -  Message Queue :
      -  The initial consideration was to use AWS SQS due to it's ease of use.
      -  Using AWS SQS would hassle-free deployment of the message queue. Also, no overheads of maintaining the queue.
      -  I started reading comparisons between popular message broker softwares (mainly AWS SQS, Kafka and RabbitMQ).
      -  It was pretty clear that Kafka has the best performance. However, Kafka is a comparatively heavy software and is most suited for high workloads. In this project, I did not expect such high workload for the message queue.
      -  AWS SQS is a good option but would incur considerable costs as I am new to message queues. Hence, I was looking for something which I could install locally. This would give me the capability of running innumerable tests without worrying about the costs.
      -  With smaller workload and costs in consideration, I decided to implement RabbitMQ
   -  API Gateway :
      -  After lab implementations using AWS API Gateway and Kong, this became a very easy decision.
      -  Kong gave me more control and is again easy to install and test locally.
   -  Database :
      -  The data to be stored for this project was structured
      -  I decided to MYSQL and created 2 tables "shortcodetolongurl" and "access_stats"
      -  "shortcodetolongurl" table is used to store the LONGURL against the SHORTCODE generated for it
      -  "access_stats" table is used to store the time as which every shortened URL was accessed
   -  Networking :
      -  This has always been my favourite part of design.
      -  I decided to create 2 private and one public subnet inside my VPC for this project
      -  As can be seen in the above diagram, all instances apart from the API Gateway are in the private subnet.
      -  This helps protect my instances against and outsider attacks
      -  Apart from separate subnets, I have used 5 different security groups for API Gateway, NoSQL Cache, LR Server, CP Server and MySQL database.
   -  Scaling :
      -  As seen in the diagram, my LR and CP servers have been configured under an auto scaling group. They will scale between 1-3 servers based on the CPU utilisation
      -  I considered applying a similar scaling to the NoSQL cache as well. However, since the NoSQL application is designed in a way where every new server needs to be made aware of the others, I decided against scaling them.
      -  Hence, the NoSQL cache is a fixed set of 5 servers running in AP mode.
   
<b>4. Challenges :</b>
   -  Creating unique hash codes -
      -  Initially I thought this would be the easiest part until I stumbled upon the problem of a "possibility" of duplicate hash codes. Eventhough I was implementing the system at a small scale, this was unacceptable.
      -  I tried to come up with my own logic of generating unique codes and also went through some blogs.
      -  After a lot of reading, I decided that the best way to deal with this was "offline generation" of hash codes or unique combinations.
      -  I generated unique 6 digit codes by having the sample set as [A-Za-z0-9]. This gave me around 62 million combinations
      -  62 million is a lot of combinations without using any special characters. I ended up generating around 25000 combinations for this project.
      -  Coupling this with multiple hostnames to serve the URL can help you scale the application with the same number of codes being used. I have demonstrated this in the video.
   -  Retrieve short code for url :
      -  The above approach of pre-generating codes also helps save some execution time when the service is live.
      -  It reduces the overhead of generatinga code on the fly. Then again, wouldn't file reads be slower? The answer was yes.
      -  This is where message queues came into the picture. I created a "producer" function which keeps pushing new codes to the queue every time one gets consumed. The queue capacity is set to 100. This means that at any given point in time, 100 new codes would be available to be used by the CP server in case of a request for URL shortening.
   -  Event Sourcing :
      -  Since I am new to message queues, this required some thought.
      -  Making sure the co-ordination between producer and consumer is correct was critical to this process.
      -  Since I was using a manually configured RabbitMQ Instance instead of AWS SQS, I had to ensure that the queues don't grow too much in size and cause the server to crash. Hence, I have also applied limits to each queue
   -  Cache Capacity :
      -  My first thoughts were to implement NoSQL cache with a capacity of about 1000 documents.
      -  After I finished the basic implementation of the CP and LR APIs, I did implement a cache capacity. Since dictionaries are unordered, I decided to delete the first key every time the cache is full and we need to make a new entry.
      -  In order to implement the capacity function, I was counting the number of keys returned by the "/api" endpoint.
      -  While running the tests, I realised that we have implemented a "soft delete" in the cache which meant that even if a key was deleted, it showed up as a key in the "/api" result.
      -  Finally, I decided not to implement a capacity for the NoSQL cache.
   -  CORS (Cross Origin Resource Sharing) :
      -  I tried creating a basic UI which takes "long url" as the input and returns the "shortened url" as output.
      -  On trying this, I ran into the CORS issue.
      -  The native "http/mux" does not support defining "handler.Methods" unlike gorilla/mux. Adding the "OPTIONS" method to this list helps to enable CORS. I did not try to resolve as I did not have enough time to move the whole project from "http/mux" to "gorilla/mux"
   -  Queue Implementation :
      -  I've used 3 queues in all :-
         -  "uniqueCodes-queue"
         -  "shortenedURL-queue"
         -  "usedURL-queue"
      -  The producers and consumers of these queues are as follows :
         ![Diagram of queue message consumption](https://github.com/akshay-sjsu-173/cmpe281/blob/master/class_projects/bitly/message-flow.pdf)
      -  The producer service which operated on the MySQL instance continuously pushed to the "uniqueCodes" queue until it was full. Then the producer waits until a code is consumed from that queue
      -  LR writes to the "usedURL" queue everytime a short link is visited
      -  CP writes to the "shortenedURL" queue every time a new link is shortened.
      -  The producer also reads from the "usedURL" queue and updates the database with access time and count every time for the URL
   -  NAT Gateway :
      -  Since lab1, I have wondered what the difference between NAT instance and AWS NAT is. NAT instance seemed to be working fine for me until it caused me a problem in lab quiz 2. After a lot of reading I found out that the problem is with the NAT instance that AWS provides us for free. It is not that well maintained. Makng a normal EC2 instance function as NAT is as simple as running 2 commands which instruct it to forward all incoming requests. I have used this NAT instance throughout this project
   -  Linux Services :
      -  I have utilised this OS level feature to ensure that all the Go applications run as systemd services
      -  For this, I have copied over the required binary or built the binary into my AMI
 

<b>5. Setting up the project :</b>
   -  AWS
      -  Deploy 5 servers and run the NoSQL java project on them. Make them aware of each other by registering against the /node endpoint
      -  Deploy a MySQL database and create required tables in it. We need two tables (shortcodetolongurl and access_stats)
      -  Deploy Kong API gateway as a standalone server in the public subnet.
      -  Create a RabbitMQ standalone or clustered setup in private subnet.
      -  Configure all Go applications to run as a service in their respective nodes.
      -  Configure Kong with separate paths for CP and LR.
   -  GCP
      -  Deploy 5 servers and run the NoSQL java project on them. Make them aware of each other by registering against the /node endpoint
      -  Deploy a MySQL database and create required tables in it. We need two tables (shortcodetolongurl and access_stats)
      -  Deploy Kong API gateway as a standalone server in the public subnet.
      -  Create a RabbitMQ cluster from Marketplace in GKE.
      -  Create a service for LR and CP (both must be of type LoadBalancer).
      -  Deploy LR and CP pods with a replicaton factor of your choice.
      -  Deploy producer and database pods.
      -  Confugure firewall rules according to deployment diagram. Make sure MySQL database as well as NoSQL cache servers are reachable from the GKE cluster.
      
<b>6. Scope for Improvements :</b>
   -  User authentication -
      -  User authentication can be added before letting anyone use the shortening service. This would be a good way towards making the service paid.
      -  With user authentication, we can improve stats to user level or provide a user with a list of all the URLs he/she has shortened so far.
   -  Clustered MySQL (Database) -
      -  The database can run as a clustered setup over for high availability.
      -  This could also help create a multi-cloud application which uses the same database.
      -  In the current scenario, eventhough the application is capable of running over multiple clouds, it runs in two different namespaces The two applications are unaware of the each other's existence.
   -  Front End :
      -  A very simple front end can be enabled for minimal functionality.
      -  This can be done by migrating to gorilla/mux library instead of http/net
   -  Config Maps for kubernetes -
      -  I have hardcoded environment variables in my Go Applications at the moment.
      -  The best way to do this would be via kubernetes envoronment variables or by passing a configuration file to the Go code.
   
<b>- Referenced Links:</b>
  - https://blog.merovius.de/2017/06/18/how-not-to-use-an-http-router.html
  - https://golang.org/pkg/net/http/
  - https://www.codementor.io/@anshulsanghi/so-you-wanna-start-developing-web-apps-with-go-huh-handle-handler-handlefunc-eziu2go2t
  - https://medium.com/linagora-engineering/how-to-choose-a-message-queue-247dde46e66c (Messge Queue)
  - https://www.educative.io/courses/grokking-the-system-design-interview/m2ygV4E81AR (Design)
  - https://medium.com/@masnun/making-http-requests-in-golang-dd123379efe7 
