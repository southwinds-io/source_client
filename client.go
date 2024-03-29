/*
  Source Configuration Service
  © 2022 Southwinds Tech Ltd - www.southwinds.io
  Licensed under the Apache License, Version 2.0 at http://www.apache.org/licenses/LICENSE-2.0
  Contributors to this project, hereby assign copyright in this code to the project,
  to be licensed under the same terms as the rest of the code.
*/

package src

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/invopop/jsonschema"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"
)

var UserAgent = fmt.Sprintf("SW-SOURCE-CLIENT-%s", Version)

type ClientOptions struct {
	InsecureSkipVerify bool
	Timeout            time.Duration
}

func (o ClientOptions) Validate() error {
	if o.Timeout < 30*time.Second {
		return fmt.Errorf("timeout must be greater than 30 secs")
	}
	return nil
}

func defaultOptions() *ClientOptions {
	return &ClientOptions{
		InsecureSkipVerify: true,
		Timeout:            60 * time.Second,
	}
}

type Client struct {
	*retryablehttp.Client
	host, token string
}

func New(host, user, pwd string, opts *ClientOptions) *Client {
	if opts == nil {
		opts = defaultOptions()
	}
	c := retryablehttp.NewClient()
	c.RetryMax = 20
	c.HTTPClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opts.InsecureSkipVerify,
			},
		},
		// set the client timeout period
		Timeout: opts.Timeout,
	}
	return &Client{ // the http client instance
		host:   host,
		token:  basicToken(user, pwd),
		Client: c,
	}
}

func (c *Client) SetType(key string, obj any) error {
	// reflects the json schema from the specified object
	schemaObj := jsonschema.Reflect(obj)
	schemaBytes, err := json.Marshal(schemaObj)
	if err != nil {
		return err
	}
	protoBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	typeInfo := &TT{
		Key:    key,
		Schema: schemaBytes,
		Proto:  protoBytes,
	}
	infoBytes, err := json.Marshal(typeInfo)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	request, err := retryablehttp.NewRequest(http.MethodPut, c.url("/type"), bytes.NewReader(infoBytes))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return reqErr
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("cannot set type, source server responded with: %s", resp.Status)
	}
	return nil
}

// Save the configuration item under the unique key using the validation defined by itemType
func (c *Client) Save(key, itemType string, item Valid) error {
	if err := item.Validate(); err != nil {
		return err
	}
	if reflect.ValueOf(item).Kind() == reflect.Ptr {
		return fmt.Errorf("item argument passed to Save() must not be a pointer")
	}
	if len(itemType) == 0 {
		return fmt.Errorf("item type is required to validate the item data")
	}
	// if the key contains a wildcard
	if strings.Contains(key, "?") {
		// generates sequence
		now := time.Now().UTC().Format("20060102150405.000")
		key = strings.Replace(key, "?", now, 1)
	}
	objBytes, err := json.Marshal(item)
	if err != nil {
		return err
	}
	request, err := retryablehttp.NewRequest(http.MethodPut, c.url("/item/%s", key), bytes.NewReader(objBytes))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	if len(itemType) > 0 {
		request.Header.Set("Source-Type", itemType)
	}
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return reqErr
	}
	if resp.StatusCode > 299 {
		var msg string
		body, err := io.ReadAll(resp.Body)
		if err == nil && len(body) > 0 {
			msg = string(body[:])
		}
		return fmt.Errorf("cannot save item, source server responded with: %s, %s", resp.Status, msg)
	}
	return nil
}

// LoadRaw the raw configuration item identified by key
func (c *Client) LoadRaw(itemKey string) (*I, error) {
	request, err := retryablehttp.NewRequest(http.MethodGet, c.url("/item/%s", itemKey), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return nil, reqErr
	}
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("cannot get item, source server responded with: %s", resp.Status)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read response body: %s", readErr)
	}
	item := new(I)
	err = json.Unmarshal(body, item)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %s", err)
	}
	return item, nil
}

// Load the typed configuration item identified by key using the specified item prototype
// The prototype is an empty instance of the type to get
func (c *Client) Load(itemKey string, prototype any) (any, error) {
	if reflect.ValueOf(prototype).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("prototype argument passed to Load() must be a pointer")
	}
	i, err := c.LoadRaw(itemKey)
	if err != nil {
		return nil, err
	}
	return i.Typed(prototype)
}

func (c *Client) LoadItemsByTagRaw(tags ...string) (IL, error) {
	request, err := retryablehttp.NewRequest(http.MethodGet, c.url("/item/tag/%s", strings.Join(tags, "|")), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return nil, reqErr
	}
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("cannot get tagged items, source server responded with: %s", resp.Status)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read response body: %s", readErr)
	}
	var items IL
	err = json.Unmarshal(body, &items)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %s", err)
	}
	return items, nil
}

func (c *Client) LoadItemsByTag(factory func() any, tags ...string) ([]any, error) {
	items, err := c.LoadItemsByTagRaw(tags...)
	if err != nil {
		return nil, err
	}
	return items.Typed(factory)
}

func (c *Client) LoadItemsByTypeRaw(itemType string) (IL, error) {
	request, err := retryablehttp.NewRequest(http.MethodGet, c.url("/item/type/%s", itemType), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return nil, reqErr
	}
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("cannot get item for type '%s', source server responded with: %s", itemType, resp.Status)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read response body: %s", readErr)
	}
	var items IL
	err = json.Unmarshal(body, &items)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %s", err)
	}
	return items, nil
}

func (c *Client) LoadItemsByType(factory func() any, itemType string) ([]any, error) {
	items, err := c.LoadItemsByTypeRaw(itemType)
	if err != nil {
		return nil, err
	}
	return items.Typed(factory)
}

func (c *Client) PopOldestRaw(itemType string) (*I, error) {
	request, err := retryablehttp.NewRequest(http.MethodDelete, c.url("/item/pop/oldest/%s", itemType), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return nil, reqErr
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("cannot get item, source server responded with: %s", resp.Status)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read response body: %s", readErr)
	}
	item := new(I)
	err = json.Unmarshal(body, item)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %s", err)
	}
	return item, nil
}

func (c *Client) PopOldest(itemType string, prototype any) (any, error) {
	if reflect.ValueOf(prototype).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("prototype argument passed to PopOldest() must be a pointer")
	}
	i, err := c.PopOldestRaw(itemType)
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, nil
	}
	return i.Typed(prototype)
}

func (c *Client) PopNewestRaw(itemType string) (*I, error) {
	request, err := retryablehttp.NewRequest(http.MethodDelete, c.url("/item/pop/newest/%s", itemType), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return nil, reqErr
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("cannot get item, source server responded with: %s", resp.Status)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read response body: %s", readErr)
	}
	item := new(I)
	err = json.Unmarshal(body, item)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %s", err)
	}
	return item, nil
}

func (c *Client) PopNewest(itemType string, prototype any) (any, error) {
	if reflect.ValueOf(prototype).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("prototype argument passed to PopNewest() must be a pointer")
	}
	i, err := c.PopOldestRaw(itemType)
	if err != nil {
		return nil, err
	}
	if i == nil {
		return i, nil
	}
	return i.Typed(prototype)
}

func (c *Client) LoadChildrenRaw(itemKey string) (IL, error) {
	request, err := retryablehttp.NewRequest(http.MethodGet, c.url("/item/%s/children", itemKey), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return nil, reqErr
	}
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("cannot get children for item, source server responded with: %s", resp.Status)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read response body: %s", readErr)
	}
	var items IL
	err = json.Unmarshal(body, &items)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %s", err)
	}
	return items, nil
}

func (c *Client) LoadChildren(factory func() any, itemKey string) ([]any, error) {
	items, err := c.LoadChildrenRaw(itemKey)
	if err != nil {
		return nil, err
	}
	return items.Typed(factory)
}

func (c *Client) LoadParentsRaw(itemKey string) (IL, error) {
	request, err := retryablehttp.NewRequest(http.MethodGet, c.url("/item/%s/parents", itemKey), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return nil, reqErr
	}
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("cannot get parents for item, source server responded with: %s", resp.Status)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read response body: %s", readErr)
	}
	var items IL
	err = json.Unmarshal(body, &items)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal response body: %s", err)
	}
	return items, nil
}

func (c *Client) LoadParents(factory func() any, itemKey string) ([]any, error) {
	items, err := c.LoadParentsRaw(itemKey)
	if err != nil {
		return nil, err
	}
	return items.Typed(factory)
}

func (c *Client) Tag(itemKey, tagName, tagValue string) error {
	var tag string
	if len(tagName) > 0 {
		if len(tagValue) > 0 {
			tag = fmt.Sprintf("%s|%s", tagName, tagValue)
		} else {
			tag = tagName
		}
	} else {
		return fmt.Errorf("a tag name is required")
	}
	request, err := retryablehttp.NewRequest(http.MethodPut, c.url("/item/%s/tag/%s", itemKey, tag), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return reqErr
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("cannot tag item, source server responded with: %s", resp.Status)
	}
	return nil
}

func (c *Client) Untag(itemKey, tagName string) error {
	if len(tagName) == 0 {
		return fmt.Errorf("a tag name is required")
	}
	request, err := retryablehttp.NewRequest(http.MethodDelete, c.url("/item/%s/tag/%s", itemKey, tagName), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return reqErr
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("cannot tag item, source server responded with: %s", resp.Status)
	}
	return nil
}

func (c *Client) Link(fromKey, toKey string) error {
	request, err := retryablehttp.NewRequest(http.MethodPut, c.url("/link/%s/to/%s", fromKey, toKey), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return reqErr
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("cannot link items, source server responded with: %s", resp.Status)
	}
	return nil
}

func (c *Client) Unlink(fromKey, toKey string) error {
	request, err := retryablehttp.NewRequest(http.MethodDelete, c.url("/link/%s/to/%s", fromKey, toKey), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return reqErr
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("cannot unlink items, source server responded with: %s", resp.Status)
	}
	return nil
}

func (c *Client) Delete(key string) error {
	request, err := retryablehttp.NewRequest(http.MethodDelete, c.url("/item/%s", key), nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", c.token)
	request.Header.Set("User-Agent", UserAgent)
	resp, reqErr := c.Do(request)
	if reqErr != nil {
		return reqErr
	}
	if resp.StatusCode > 299 {
		return fmt.Errorf("cannot delete item, source server responded with: %s", resp.Status)
	}
	return nil
}

func (c *Client) url(format string, args ...any) string {
	v := fmt.Sprintf("%s%s", c.host, fmt.Sprintf(format, args...))
	return v
}

func basicToken(user string, pwd string) string {
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", user, pwd))))
}
