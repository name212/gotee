//go:build !(first && second)

package main

func getTagStr() string {
	return "no tags"
}
