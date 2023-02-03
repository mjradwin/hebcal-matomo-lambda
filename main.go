package main

import (
	"context"
	"crypto/md5"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/twmb/murmur3"
)

type UserLocation struct {
	ZipCode   string
	Latitude  float64
	Longitude float64
	Tzid      string
	Name      string
	State     string
	Cc        string
	CityName  string
}

type EventDetails struct {
	Category string
	Action   string
	Name     string
}

type TrackingMessage struct {
	Timestamp   string
	Client      string
	RequestType string
	RequestId   string
	SessionId   string
	UserId      string
	Locale      string
	IntentName  string
	Duration    int
	Title       string
	Slots       map[string]string `json:"slots,omitempty"`
	Location    *UserLocation     `json:"location,omitempty"`
	Details     *EventDetails     `json:"details,omitempty"`
}

func getItentName(msg TrackingMessage) string {
	if msg.IntentName != "" {
		return msg.IntentName
	}
	return msg.RequestType
}

func getActionName(msg TrackingMessage) string {
	if msg.Title != "" {
		return msg.Title
	}
	return getItentName(msg)
}

func getPageviewId(requestId string) string {
	hash := murmur3.StringSum32(requestId)
	bytes := make([]byte, 4)
	bytes[0] = byte(hash & 0xff)
	bytes[1] = byte((hash >> 8) & 0xff)
	bytes[2] = byte((hash >> 16) & 0xff)
	bytes[3] = byte((hash >> 24) & 0xff)
	str := b64.RawURLEncoding.EncodeToString(bytes)
	str = strings.ReplaceAll(str, "-", "x")
	str = strings.ReplaceAll(str, "_", "y")
	return str
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	for _, message := range sqsEvent.Records {
		var msg TrackingMessage
		json.Unmarshal([]byte(message.Body), &msg)

		fmt.Printf("sqsMessageId=%s,requestType=%s,requestId=%s,intent=%s,userId=%s\n",
			message.MessageId, msg.RequestType, msg.RequestId, msg.IntentName, msg.UserId)

		client := &http.Client{}
		req, err := http.NewRequest("GET", "http://www.hebcal.com/ma/ma.php", nil)
		if err != nil {
			return err
		}

		q := req.URL.Query()
		q.Add("rec", "1")
		q.Add("apiv", "1")
		q.Add("idsite", "4")
		q.Add("send_image", "0") // prefer HTTP 204 instead of a GIF image
		q.Add("lang", msg.Locale)

		if msg.RequestId != "" {
			q.Add("pv_id", getPageviewId(msg.RequestId))
		}

		if msg.Duration != 0 {
			q.Add("pf_srv", strconv.Itoa(msg.Duration))
		}

		actionName := getActionName(msg)
		q.Add("action_name", actionName)

		path := "http://alexa.hebcal.com/" + getItentName(msg)
		for slot, val := range msg.Slots {
			path += "/" + slot + "/" + url.QueryEscape(val)
		}
		q.Add("url", path)

		if msg.UserId != "" {
			data := []byte(msg.UserId)
			bytes := md5.Sum(data)
			bytes[6] = (bytes[6] & 0x0f) | 0x40
			bytes[8] = (bytes[8] & 0x3f) | 0x80
			uid := fmt.Sprintf("%x-%x-%x-%x-%x",
				bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
			q.Add("uid", uid)
			vid := fmt.Sprintf("%x%x", bytes[0:2], bytes[10:16])
			q.Add("_id", vid)
			q.Add("cid", vid)
		}

		if msg.Details != nil {
			evt := msg.Details
			if evt.Category != "" {
				q.Add("e_c", evt.Category)
			}
			if evt.Action != "" {
				q.Add("e_a", evt.Action)
			}
			if evt.Name != "" {
				q.Add("e_n", evt.Name)
			}
		}

		matomoToken := os.Getenv("MATOMO_TOKEN")
		if matomoToken != "" && msg.Location != nil {
			q.Add("token_auth", matomoToken)
			loc := msg.Location
			if loc.Cc != "" {
				q.Add("country", strings.ToLower(loc.Cc))
			}
			if loc.Cc == "US" {
				if loc.State != "" {
					q.Add("region", loc.State)
				} else if loc.CityName != "" {
					re := regexp.MustCompile(`, [A-Z][A-Z]$`)
					cityName := loc.CityName
					if re.MatchString(cityName) {
						length := len(cityName)
						state := cityName[length-2 : length]
						q.Add("region", state)
						loc.Name = cityName[0 : length-4]
					}
				}
			}
			cityName := loc.Name
			if cityName == "" {
				cityName = loc.CityName
			}
			if cityName != "" {
				q.Add("city", cityName)
			}
			q.Add("lat", fmt.Sprintf("%f", loc.Latitude))
			q.Add("long", fmt.Sprintf("%f", loc.Longitude))
		}

		req.URL.RawQuery = q.Encode()

		fmt.Println(req.URL.String())

		req.Header.Add("User-Agent", msg.Client)
		req.Header.Add("X-Forwarded-Proto", "https")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 {
			return errors.New("Unexpected status code " + strconv.Itoa(resp.StatusCode))
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
