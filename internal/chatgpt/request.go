package chatgpt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"freechatgpt/typings"
	chatgpt_types "freechatgpt/typings/chatgpt"
	"io"
	"math/rand"
	"os"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/gin-gonic/gin"

	chatgpt_response_converter "freechatgpt/conversion/response/chatgpt"

	// chatgpt "freechatgpt/internal/chatgpt"

	official_types "freechatgpt/typings/official"
)

var proxies []string

var (
	jar     = tls_client.NewCookieJar()
	options = []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(360),
		tls_client.WithClientProfile(tls_client.Safari_Ipad_15_6),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar), // create cookieJar instance and pass it as argument
		// Disable SSL verification
		tls_client.WithInsecureSkipVerify(),
	}
	client, _         = tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	http_proxy        = os.Getenv("http_proxy")
	API_REVERSE_PROXY = os.Getenv("API_REVERSE_PROXY")
)

func init() {
	// Check for proxies.txt
	if _, err := os.Stat("proxies.txt"); err == nil {
		// Each line is a proxy, put in proxies array
		file, _ := os.Open("proxies.txt")
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			// Split line by :
			proxy := scanner.Text()
			proxy_parts := strings.Split(proxy, ":")
			if len(proxy_parts) == 2 {
				proxy = "socks5://" + proxy
			} else if len(proxy_parts) == 4 {
				proxy = "socks5://" + proxy_parts[2] + ":" + proxy_parts[3] + "@" + proxy_parts[0] + ":" + proxy_parts[1]
			} else {
				continue
			}
			proxies = append(proxies, proxy)
		}
	}
}

func random_int(min int, max int) int {
	return min + rand.Intn(max-min)
}

type arkose_response struct {
	Token string `json:"token"`
}

func Get_arkose_token() (string, error) {
	resp, err := client.Get("http://bypass.churchless.tech/api/arkose")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	// println(string(payload))
	url := "https://tcr9i.chat.openai.com/fc/gt2/public_key/35536E1E-65B4-4D96-9D97-6ADB7EFF8147"
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	req.Header.Set("Host", "tcr9i.chat.openai.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:114.0) Gecko/20100101 Firefox/114.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	// form
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://tcr9i.chat.openai.com")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://tcr9i.chat.openai.com/v2/1.5.2/enforcement.64b3a4e29686f93d52816249ecbf9857.html")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("TE", "trailers")
	resp, err = client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var arkose arkose_response
	err = json.NewDecoder(resp.Body).Decode(&arkose)
	if err != nil {
		return "", err
	}
	println(arkose.Token)
	return arkose.Token, nil
}

func POSTconversation(message chatgpt_types.ChatGPTRequest, access_token string) (*http.Response, error) {
	if http_proxy != "" && len(proxies) == 0 {
		client.SetProxy(http_proxy)
	}
	// Take random proxy from proxies.txt
	if len(proxies) > 0 {
		client.SetProxy(proxies[random_int(0, len(proxies))])
	}

	apiUrl := "https://chat.openai.com/backend-api/conversation"
	if API_REVERSE_PROXY != "" {
		apiUrl = API_REVERSE_PROXY
	}

	// JSONify the body and add it to the request
	body_json, err := json.Marshal(message)
	if err != nil {
		return &http.Response{}, err
	}

	request, err := http.NewRequest(http.MethodPost, apiUrl, bytes.NewBuffer(body_json))
	if err != nil {
		return &http.Response{}, err
	}
	// Clear cookies
	if os.Getenv("PUID") != "" {
		request.Header.Set("Cookie", "_puid="+os.Getenv("PUID")+";")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
	request.Header.Set("Accept", "*/*")
	if access_token != "" {
		request.Header.Set("Authorization", "Bearer "+access_token)
	}
	if err != nil {
		return &http.Response{}, err
	}
	response, err := client.Do(request)
	return response, err
}

// Returns whether an error was handled
func Handle_request_error(c *gin.Context, response *http.Response) bool {
	if response.StatusCode != 200 {
		// Try read response body as JSON
		var error_response map[string]interface{}
		err := json.NewDecoder(response.Body).Decode(&error_response)
		if err != nil {
			// Read response body
			body, _ := io.ReadAll(response.Body)
			c.JSON(500, gin.H{"error": gin.H{
				"message": "Unknown error",
				"type":    "internal_server_error",
				"param":   nil,
				"code":    "500",
				"details": string(body),
			}})
			return true
		}
		c.JSON(response.StatusCode, gin.H{"error": gin.H{
			"message": error_response["detail"],
			"type":    response.Status,
			"param":   nil,
			"code":    "error",
		}})
		return true
	}
	return false
}

type ContinueInfo struct {
	ConversationID string `json:"conversation_id"`
	ParentID       string `json:"parent_id"`
}

func Handler(c *gin.Context, response *http.Response, token string, translated_request chatgpt_types.ChatGPTRequest, stream bool) (string, *ContinueInfo) {
	max_tokens := false

	// Create a bufio.Reader from the response body
	reader := bufio.NewReader(response.Body)

	// Read the response byte by byte until a newline character is encountered
	if stream {
		// Response content type is text/event-stream
		c.Header("Content-Type", "text/event-stream")
	} else {
		// Response content type is application/json
		c.Header("Content-Type", "application/json")
	}
	var finish_reason string
	var previous_text typings.StringStruct
	var original_response chatgpt_types.ChatGPTResponse
	var isRole = true
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", nil
		}
		if len(line) < 6 {
			continue
		}
		// Remove "data: " from the beginning of the line
		line = line[6:]
		// Check if line starts with [DONE]
		if !strings.HasPrefix(line, "[DONE]") {
			// Parse the line as JSON

			err = json.Unmarshal([]byte(line), &original_response)
			if err != nil {
				continue
			}
			if original_response.Error != nil {
				c.JSON(500, gin.H{"error": original_response.Error})
				return "", nil
			}
			if original_response.Message.Author.Role != "assistant" || original_response.Message.Content.Parts == nil {
				continue
			}
			if original_response.Message.Metadata.MessageType != "next" && original_response.Message.Metadata.MessageType != "continue" || original_response.Message.EndTurn != nil {
				continue
			}
			response_string := chatgpt_response_converter.ConvertToString(&original_response, &previous_text, isRole)
			isRole = false
			if stream {
				_, err = c.Writer.WriteString(response_string)
				if err != nil {
					return "", nil
				}
			}
			// Flush the response writer buffer to ensure that the client receives each line as it's written
			c.Writer.Flush()

			if original_response.Message.Metadata.FinishDetails != nil {
				if original_response.Message.Metadata.FinishDetails.Type == "max_tokens" {
					max_tokens = true
				}
				finish_reason = original_response.Message.Metadata.FinishDetails.Type
			}

		} else {
			if stream {
				final_line := official_types.StopChunk(finish_reason)
				c.Writer.WriteString("data: " + final_line.String() + "\n\n")
			}
		}
	}
	if !max_tokens {
		return previous_text.Text, nil
	}
	return previous_text.Text, &ContinueInfo{
		ConversationID: original_response.ConversationID,
		ParentID:       original_response.Message.ID,
	}
}

func GETengines() (interface{}, int, error) {
	url := "https://api.openai.com/v1/models"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+os.Getenv("OFFICIAL_API_KEY"))
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, resp.StatusCode, nil
}
