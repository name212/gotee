//go:build !dynamic

package main

func getUser() string {
	return "Not dynamic"
}
