package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	masterURL string
	client    *http.Client
}

func NewClient(masterURL string) *Client {
	return &Client{
		masterURL: strings.TrimRight(masterURL, "/"),
		client:    &http.Client{}, // default client follows redirects
	}
}

// put uploads a blob with the given key
func (c *Client) Put(key string, data []byte) error {
	url := fmt.Sprintf("%s/blob/%s", c.masterURL, key)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(data))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("put failed: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// get retrieves a blob
func (c *Client) Get(key string) ([]byte, error) {
	url := fmt.Sprintf("%s/blob/%s", c.masterURL, key)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("blob not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get failed: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// delete removes a blob
func (c *Client) Delete(key string) error {
	url := fmt.Sprintf("%s/blob/%s", c.masterURL, key)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete failed: status %d", resp.StatusCode)
	}

	return nil
}
