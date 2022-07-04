# WiFi_Positioning_Server_go

<b>Golang</b> conversion of Gachon Univefrsity Sensor and Wireless Network Class Term Project.<br>
This is a project for the experimental implementation of WiFi Indoor Positioning technology and golang.

We used WiFi Fingerprinting technique to implement WiFi Positioning. This is for more general use as there are APs that do not yet support WiFi RTT.

Original version is developed with Node.js<br>
https://github.com/chromato99/WiFi_Positioning_Server

This is a repository for server, and the client implementation can be found at the link below.<br>
Client App : https://github.com/chromato99/WiFi_Positining 

# Server App

This server was written in Go language and developed in the form of RESTful API using Gin Web Framework.

The project structure consists of main, core module, and result module.

Main is written for the entry point when a client connects, and the main processing is written in the core module. In the result module, a struct containing result data and a priority queue used in the result list are defined.

### Run Server

```zsh
# Set password
cd <project dir>/generator
go build password-generator.go
./password-generator

# Run server with docker
cd <project dir>
docker build --tag wifi-pos-sever
docker run -d -p 8004:8004 wifi-pos-server
```

# Implementation

We used mysql database and implemented two functions except for test.

### Database

To implement the database, you need to create a DB and table in the database, and set the connection in the db-config.json file.

Database Table<br>
<img src="https://user-images.githubusercontent.com/20539422/175896908-c36c2f7d-cad9-432f-b3b5-f589cffd781f.png" width=70% height="70%">

db-config.json (You can check it in the db-config.template.json file.)
```json
{
    "User": "example",
    "Passwd": "example",
    "Net": "tcp",
    "Addr": "0.0.0.0:3306",
    "DBName": "WiFi_Pos_go"
}
```

### /add

This is a function that adds data to the database. It receives data in json format and inserts it into the database. In addition, a password function was implemented to prevent anyone from adding data and to prevent the case of adding data incorrectly.

Input Data Format (JSON)<br>
```json
{
    "position" : "Gachon AI 311",
    "password" : "password"
    "wifi_data" : [
        {
            "bssid" : "xx:xx:xx:xx:xx:xx",
            "rssi" : -60
        },
        {
            "bssid" : "xx:xx:xx:xx:xx:xx",
            "rssi" : -30
        },
        {
            "bssid" : "xx:xx:xx:xx:xx:xx",
            "rssi" : -55
        },
    ]
}
```

For password, you can get the encrypted password as password.json file by executing the password-generator.go code in the generator directory.

### /findPosition

This function receives input data and estimates the current location by comparing it with data previously stored in the database.

Firstly, the comparison is done in a brute force way with all data in the database. So, for speed, it uses a multi-threading technique. In the comparison process, the difference between the data is calculated by subtracting the rssi value. (In this case, only data with the same bssid are compared.)

Then, add up all the differences of the rssi values and divide by the number of the same bssid to find the average difference. 
Lastly, it is further divided by the count value, which is to reflect the number of the same bssid in the value. 

After that, the final result is derived by comparing the four values in the order of the smallest among these finally calculated values. (In this process, priority queue is used.)

# Tech Stack

Go<br>
Gin Web Framework<br>
MySQL
