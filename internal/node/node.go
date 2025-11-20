package node

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/sanchey92/go-cube/internal/services/stats"
	"github.com/sanchey92/go-cube/pkg/utils"
)

type Node struct {
	Name            string
	IP              string
	API             string
	Cores           int64
	Memory          int64
	MemoryAllocated int64
	Disk            int64
	DiskAllocated   int64
	Stats           *stats.Stats
	Role            string
	TaskCount       int
}

func NewNode(name, api, role string) *Node {
	return &Node{
		Name: name,
		API:  api,
		Role: role,
	}
}

func (n *Node) GetStats() (*stats.Stats, error) {
	var resp *http.Response
	var err error

	url := fmt.Sprintf("%s/stats", n.API)
	resp, err = utils.HTTPWithTRetry(http.Get, url)
	if err != nil {
		msg := fmt.Sprintf("unable to connect to %v. Permanent failure.\n", n.API)
		log.Printf(msg)
		return nil, errors.New(msg)
	}

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error retrieving stats from %v: %v", n.API, err)
		log.Println(msg)
		return nil, errors.New(msg)
	}

	defer resp.Body.Close()

	return n.Stats, nil
}
