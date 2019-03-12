package common

import (
	"log"
	"os/user"
)

var CurrentUser *user.User

func init() {
	var err error
	CurrentUser, err = user.Current()
	if err != nil {
		log.Fatalln("Get current user failed:", err)
	}
}
