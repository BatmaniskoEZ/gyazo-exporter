package main

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
)

type ImageAPIResponse struct {
	URL     string `json:"url"`
	ImageID string `json:"image_id"`

	Metadata struct {
		App string `json:"app"`
	} `json:"metadata"`

	CreatedAt string `json:"created_at"`
	Type      string `json:"type"`
}

func main() {
	// run with pwsh $env:GYAZO_ACCESS_TOKEN="xyz" go run .
	accessToken := os.Getenv("GYAZO_ACCESS_TOKEN")

	if accessToken == "" {
		log.Fatal("access_token is required")
	}

	// not sure if correct xd
	if err := os.Mkdir("images", 0777); err != nil {
		if !errors.Is(err, os.ErrExist){ // If the folder exists already its nothing to be worried about
			log.Fatal(err)
		}
	}
	
	downloadClient := grab.NewClient()
	httpClient := &http.Client{}

	images := requestImages(httpClient, &accessToken)
	fmt.Println("Found", len(images), "images")

	for len(images) != 0 {
		for _, image := range images {
			if image.URL == "" { // For some reason non premium API will give empty responses
				continue
			}

			originalfileName := getNewFileName(&image)
			var re = regexp.MustCompile("[\\\\/:*?\"<>|]")
			fileName := re.ReplaceAllString(originalfileName, "") //rename for win

			fmt.Println("Processing...", originalfileName, "new filename...", fileName)
			req, err := grab.NewRequest("./images/"+fileName, image.URL)
			if err != nil {
				log.Fatal(err)
			}

			resp := downloadClient.Do(req)

			t := time.NewTicker(500 * time.Millisecond)
			defer t.Stop()

		Loop:
			for {
				select {
				case <-t.C:
					fmt.Printf("Transferred %v / %v bytes (%.2f%%)\n", resp.BytesComplete(), resp.Size(), 100*resp.Progress())
				case <-resp.Done:
					break Loop
				}
			}

			// check for errors
			if err = resp.Err(); err != nil {
				fmt.Println("Download failed ❌")
				log.Panic(err)
			}

			fmt.Println("Successfully downloaded ✅")
			deleteImage(httpClient, &accessToken, &image.ImageID)
			fmt.Println("Successfully deleted from gyazo ✅")
		}

		images = requestImages(httpClient, &accessToken)
		fmt.Println("Found", len(images), "images")
	}

	fmt.Println("Finished, have a nice day! :)")
}

// Requests image api
func requestImages(client *http.Client, accessToken *string) []ImageAPIResponse {
	resp, err := client.Get("https://api.gyazo.com/api/images?per_page=100&access_token=" + *accessToken)
	if err != nil {
		log.Fatal(err)
	}

	jsonRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var images []ImageAPIResponse
	json.Unmarshal(jsonRaw, &images)

	return images
}

// Requests image deletion api
func deleteImage(client *http.Client, accessToken *string, imageID *string) {
	req, err := http.NewRequest("DELETE", "https://api.gyazo.com/api/images/"+*imageID+"?access_token="+*accessToken, nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != 200 {
		log.Fatal("Error deleting image ❌")
	}
}

// Creates a new nice filename from the metadata
func getNewFileName(image *ImageAPIResponse) string {
	created_at := image.CreatedAt[:len(image.CreatedAt)-5]
	raw_name := strings.TrimSpace(image.Metadata.App)
	if raw_name != "" {
		name := strings.ReplaceAll(raw_name, " ", "_")
		return fmt.Sprintf("%s_%s.%s", name, created_at, image.Type)
	} else {
		return fmt.Sprintf("%s.%s", created_at, image.Type)
	}
}
