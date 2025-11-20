package node

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/sanchey92/go-cube/internal/services/stats"
	"github.com/sanchey92/go-cube/pkg/errors"
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
		return nil, errors.ErrUnableConnectToAPI
	}

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error retrieving stats from %v: %v", n.API, err)
		log.Println(msg)
		return nil, errors.ErrRetrievingStats
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var s stats.Stats

	if err = json.Unmarshal(body, &s); err != nil {
		msg := fmt.Sprintf("error decoding message while getting stats for node %s", n.Name)
		log.Println(msg)
		return nil, errors.ErrDecodingMessage
	}

	if s.MemStats == nil || s.DiskStats == nil {
		return nil, errors.ErrRetrievingStats
	}

	n.Memory = int64(s.MemTotalKb())
	n.Disk = int64(s.DiskTotal())
	n.Stats = &s

	return n.Stats, nil
}
