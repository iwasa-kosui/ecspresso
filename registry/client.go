package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// Repositry represents DockeHub repositry
type Repositry struct {
	client   *http.Client
	host     string
	repo     string
	user     string
	password string
	token    string
}

// New creates a client for a repositry.
func New(image, user, password string) *Repositry {
	c := &Repositry{
		client:   &http.Client{},
		user:     user,
		password: password,
	}
	p := strings.SplitN(image, "/", 2)
	host := p[0]
	if strings.Contains(host, ".") {
		// Docker registry v2 API
		c.host = host
		c.repo = p[1]
	} else {
		// DockerHub
		if !strings.Contains(image, "/") {
			image = "library/" + image
		}
		c.host = "registry.hub.docker.com"
		c.repo = image
	}
	return c
}

func (c *Repositry) login(endpoint, service, scope string) error {
	u := fmt.Sprintf("%s?service=%s&scope=%s", endpoint, service, scope)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	if c.user != "" && c.password != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("login failed %s", resp.Status)
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	var body struct {
		Token string
	}
	if err := dec.Decode(&body); err != nil {
		return err
	}
	if body.Token == "" {
		return errors.New("response does not contains token")
	}
	c.token = body.Token
	return nil
}

func (c *Repositry) getManifests(tag string) (*http.Response, error) {
	u := fmt.Sprintf("https://%s/v2/%s/manifests/%s", c.host, c.repo, tag)
	req, _ := http.NewRequest(http.MethodHead, u, nil)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.user == "AWS" && c.password != "" {
		// ECR
		req.Header.Set("Authorization", "Basic "+c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return resp, err
}

// HasImage returns an image tag exists or not in the repository.
func (c *Repositry) HasImage(tag string) (bool, error) {
	tries := 2
	for tries > 0 {
		tries--
		resp, err := c.getManifests(tag)
		if err != nil {
			return false, err
		}
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			h := resp.Header.Get("Www-Authenticate")
			if strings.HasPrefix(h, "Bearer ") {
				auth := strings.SplitN(h, " ", 2)[1]
				if err := c.login(parseAuthHeader(auth)); err != nil {
					return false, err
				}
			}
		case http.StatusOK:
			return true, nil
		default:
			return false, errors.New(resp.Status)
		}
	}
	return false, errors.New("aborted")
}

var (
	partRegexp = regexp.MustCompile(`([a-zA-Z0-9_]+)="([^"]*)"`)
)

func parseAuthHeader(bearer string) (endpoint, service, scope string) {
	parsed := make(map[string]string, 3)
	for _, part := range partRegexp.FindAllString(bearer, -1) {
		kv := strings.SplitN(part, "=", 2)
		parsed[kv[0]] = kv[1][1 : len(kv[1])-1]
	}
	return parsed["realm"], parsed["service"], parsed["scope"]
}
