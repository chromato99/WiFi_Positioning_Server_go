package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/bcrypt"
)

type Passwd struct {
	Key string `json:"key"`
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter new password: ")
	text, _ := reader.ReadString('\n')

	// Convert to password excluding \n character at the end
	pw, err := bcrypt.GenerateFromPassword([]byte(text[:len(text)-1]), bcrypt.DefaultCost)

	if err != nil {
		fmt.Println(err)
	}

	var passwd Passwd

	passwd.Key = string(pw)
	fmt.Println(passwd.Key)

	file, err2 := json.Marshal(passwd)
	if err2 != nil {
		fmt.Println(err2)
	}

	err3 := ioutil.WriteFile("../core/password.json", file, 0644)
	if err3 != nil {
		fmt.Println(err3)
	}
}
