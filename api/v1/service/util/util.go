package util

import (
	"os"
	"regexp"

	"github.com/NetSepio/erebrus/model"
	log "github.com/sirupsen/logrus"
)

var IsLetter = regexp.MustCompile(`^[a-z0-9]+$`).MatchString

// StandardFields for logger
var StandardFields = log.Fields{
	"hostname": "HostServer",
	"appname":  "ServiceAPI",
}

// ReadFile file content
func ReadFile(path string) (bytes []byte, err error) {
	bytes, err = os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// WriteFile content to file
func WriteFile(path string, bytes []byte) (err error) {
	err = os.WriteFile(path, bytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

// FileExists check if file exists
func FileExists(name string) bool {
	info, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func CreateJSONFile(path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	_, err = file.Write([]byte("[]"))
	if err != nil {
		return err
	}

	return nil
}

// CheckError for checking any errors
func CheckError(message string, err error) {
	if err != nil {
		log.WithFields(StandardFields).Fatalf("%s %+v", message, err)
	}
}

// LogErrors for checking any errors
func LogError(message string, err error) {
	if err != nil {
		log.WithFields(StandardFields).Warnf("%s %+v", message, err)
	}
}

// Message Return Response as map
func Message(status int, message string) map[string]interface{} {
	return map[string]interface{}{"status": status, "message": message}
}

// MessageByte Return Response as byte array
func MessageService(status int, message model.Service) map[string]interface{} {
	return map[string]interface{}{"status": status, "message": message}
}

// MessageByte Return Response as byte array
func MessageServices(status int, message []model.Service) map[string]interface{} {
	return map[string]interface{}{"status": status, "message": message}
}
