//go:build dynamic

package main

import (
	"fmt"
	"os/user"
)

func getUser() string {
	u, err := user.Current()
	if err != nil {
		return fmt.Sprintf("ERROR: %s", err.Error())
	}
	return u.Name
}