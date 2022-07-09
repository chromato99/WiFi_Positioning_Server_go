package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/chromato99/WiFi_Positioning_Server_go/core"

	"github.com/gin-gonic/gin"
)

func main() {
	threadNum, err := strconv.Atoi(os.Getenv("THREAD_NUM"))
	if err != nil {
		fmt.Println(err)
	}
	runtime.GOMAXPROCS(threadNum)

	router := gin.Default()
	router.POST("/test", core.Test)
	router.POST("/add", core.AddData)
	router.POST("/findPosition", core.FindPosition)

	router.Run(":8004")
}
