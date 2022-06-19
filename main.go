package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

type PosData struct {
	Position string     `json:"position"`
	Password string     `json:"password"`
	WifiData []WifiData `json:"wifi_data"`
}

type WifiData struct {
	Mac string `json:"mac"`
	Rss int    `json:"rss"`
}

type ResultData struct {
	Id       int
	Position string
	Count    int
	Avg      float64
	Ratio    float64
}

type DBData struct {
	Id       int
	Position string
	WifiData []WifiData
}

func main() {
	router := gin.Default()
	router.POST("/test", test)
	router.POST("/add", addData)
	router.POST("/findPosition", findPosition)

	router.Run(":80")
}

// test 용 합수 수신받은 와이파이 데이터를 반환해준자
func test(c *gin.Context) {
	rawData, err := c.GetRawData()
	if err != nil {
		//Handle Error
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}
	var data PosData
	json.Unmarshal([]byte(rawData), &data)
	doc, _ := json.Marshal(data.WifiData)

	c.IndentedJSON(http.StatusOK, string(doc))
}

// Add wifi location data to database
func addData(c *gin.Context) {

	// Process the received json data
	rawData, err := c.GetRawData()
	if err != nil {
		//Handle Error
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}
	var newData PosData
	json.Unmarshal([]byte(rawData), &newData)
	fmt.Println(newData)
	wifi_data, _ := json.Marshal(newData.WifiData)

	// Reading db configuration information file
	db_config_file, err := os.Open("./src/db-config.json")
	if err != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}

	db_config_byte, err := ioutil.ReadAll(db_config_file)
	if err != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}

	// Convert json format to golang map format
	var db_config map[string]string
	json.Unmarshal(db_config_byte, &db_config)

	// db settings
	cfg := mysql.Config{
		User:                 db_config["User"],
		Passwd:               db_config["Passwd"],
		Net:                  db_config["Net"],
		Addr:                 db_config["Addr"],
		DBName:               db_config["DBName"],
		AllowNativePasswords: true,
	}

	// open mysql db
	db, dberror := sql.Open("mysql", cfg.FormatDSN())
	if dberror != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}

	// insert wifi position data
	result, err := db.Exec("INSERT INTO wifi_data (position, wifi_data) VALUES (?, ?)", newData.Position, string(wifi_data))
	if err != nil {
		fmt.Println(err)
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Insert Error Ocurred!!",
		})
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}

	// when insert data sucessed
	c.IndentedJSON(http.StatusOK, gin.H{
		"status":     "success",
		"insertedId": id,
	})
}

func findPosition(c *gin.Context) {
	// Process the received json data
	rawData, err := c.GetRawData()
	if err != nil {
		//Handle Error
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}
	var newData PosData
	json.Unmarshal([]byte(rawData), &newData)

	// Reading db configuration information file
	db_config_file, err := os.Open("./src/db-config.json")
	if err != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}

	db_config_byte, err := ioutil.ReadAll(db_config_file)
	if err != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}

	// Convert json format to golang map format
	var db_config map[string]string
	json.Unmarshal(db_config_byte, &db_config)

	// db settings
	cfg := mysql.Config{
		User:                 db_config["User"],
		Passwd:               db_config["Passwd"],
		Net:                  db_config["Net"],
		Addr:                 db_config["Addr"],
		DBName:               db_config["DBName"],
		AllowNativePasswords: true,
	}

	// open mysql db
	db, dberror := sql.Open("mysql", cfg.FormatDSN())
	if dberror != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}

	// get all wifi position data from db
	rows, err := db.Query("SELECT * FROM wifi_data")
	if dberror != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Error Ocurred!!",
		})
		return
	}

	// Append received db data to array
	var db_pos_arr []DBData
	for rows.Next() {
		var db_pos DBData
		var raw_wifi_data string
		if err := rows.Scan(&db_pos.Id, &db_pos.Position, &raw_wifi_data); err != nil {
			fmt.Printf("DB Scan Error : %v", err)
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "DB Scan Error!",
			})
			return
		}
		json.Unmarshal([]byte(raw_wifi_data), &db_pos.WifiData)
		db_pos_arr = append(db_pos_arr, db_pos)
	}

	// The part that calls the position estimation operation
	ch := make(chan []ResultData)
	slice_len := int(math.Ceil(float64(len(db_pos_arr)) / 3))

	for i := 0; i < 2; i++ {
		go calcPos(db_pos_arr[slice_len*i:slice_len*(i+1)], newData, 0.6, ch)
	}
	go calcPos(db_pos_arr[slice_len*2:], newData, 0.6, ch)

	var result_arr []ResultData
	for i := 0; i < 3; i++ {
		result_arr = append(result_arr, <-ch...)
	}

	best_result := calcKnn(result_arr, 4)

	// respond best position result
	c.IndentedJSON(http.StatusOK, gin.H{
		"Position": best_result.Position,
		"k_count":  best_result.Count,
	})
}

func calcPos(DBPos []DBData, inputPos PosData, margin float64, ch chan []ResultData) {
	var result_arr []ResultData

	for _, pos := range DBPos {
		result := ResultData{Id: pos.Id, Position: pos.Position, Count: 0, Avg: 0, Ratio: 0}
		var sum int = 0

		for _, wifi_data := range pos.WifiData {
			for _, input_wifi := range inputPos.WifiData {
				if wifi_data.Mac == input_wifi.Mac {
					result.Count++
					sum += int(math.Abs(float64(wifi_data.Rss) - float64(input_wifi.Rss)))
					break
				}
			}
		}
		result.Avg = float64(sum) / float64(result.Count)
		result.Ratio = result.Avg / float64(result.Count)
		result_arr = append(result_arr, result)
	}

	sort.Slice(result_arr, func(i, j int) bool {
		return result_arr[i].Count > result_arr[j].Count
	})
	largest_count := result_arr[0].Count
	var top_result_arr []ResultData
	for i := 0; i < len(result_arr) && float64(result_arr[i].Count) > (float64(largest_count)*margin); i++ {
		top_result_arr = append(top_result_arr, result_arr[i])
	}
	sort.Slice(top_result_arr, func(i, j int) bool {
		return top_result_arr[i].Ratio < top_result_arr[j].Ratio
	})

	ch <- top_result_arr
}

func calcKnn(result_arr []ResultData, k int) ResultData {
	k_count := make(map[string]int)
	for i := 0; i < k && i < len(result_arr); i++ {
		if _, ok := k_count[result_arr[i].Position]; ok {
			k_count[result_arr[i].Position]++
		} else {
			k_count[result_arr[i].Position] = 1
		}
	}

	best_result := ResultData{
		Id:       0,
		Position: "not found",
		Count:    0,
		Avg:      0,
		Ratio:    0,
	}

	for key, value := range k_count {
		if value > best_result.Count {
			best_result.Count = value
			best_result.Position = key
		} else if value == best_result.Count && key == result_arr[0].Position {
			best_result.Count = value
			best_result.Position = key
		}
	}

	fmt.Println(result_arr[:k])

	return best_result
}
