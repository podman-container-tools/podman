//go:build windows

package hypervctl

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/containers/libhvee/pkg/wmiext"
)

const (
	KvpOperationFailed    = 32768
	KvpAccessDenied       = 32769
	KvpNotSupported       = 32770
	KvpStatusUnknown      = 32771
	KvpTimeoutOccurred    = 32772
	KvpIllegalArgument    = 32773
	KvpSystemInUse        = 32774
	KvpInvalidState       = 32775
	KvpIncorrectDataType  = 32776
	KvpSystemNotAvailable = 32777
	KvpOutOfMemory        = 32778
	KvpNotFound           = 32779

	KvpExchangeDataItemName = "Msvm_KvpExchangeDataItem"
	MemorySettingDataName   = "Msvm_MemorySettingData"
)

type CimKvpItems struct {
	Instances []CimKvpItem `xml:"INSTANCE"`
}

type CimKvpItem struct {
	Properties []CimKvpItemProperty `xml:"PROPERTY"`
}

type CimKvpItemProperty struct {
	Name  string `xml:"NAME,attr"`
	Value string `xml:"VALUE"`
}

type KvpError struct {
	ErrorCode int
	message   string
}

func (k *KvpError) Error() string {
	return fmt.Sprintf("%s (%d)", k.message, k.ErrorCode)
}

func createKvpItem(service *wmiext.Service, key string, value string) (string, error) {
	item, err := service.SpawnInstance(KvpExchangeDataItemName)
	if err != nil {
		return "", err
	}
	defer item.Close()

	_ = item.Put("Name", key)
	_ = item.Put("Data", value)
	_ = item.Put("Source", 0)
	itemStr := item.GetCimText()
	return itemStr, nil
}

func parseKvpMapXml(kvpXml string) (map[string]string, error) {
	// Workaround XML decoder's inability to handle multiple root elements
	r := io.MultiReader(
		strings.NewReader("<root>"),
		strings.NewReader(kvpXml),
		strings.NewReader("</root>"),
	)

	var items CimKvpItems
	if err := xml.NewDecoder(r).Decode(&items); err != nil {
		return nil, err
	}

	ret := make(map[string]string)
	for _, item := range items.Instances {
		var key, value string
		for _, prop := range item.Properties {
			if strings.EqualFold(prop.Name, "Name") {
				key = prop.Value
			} else if strings.EqualFold(prop.Name, "Data") {
				value = prop.Value
			}
		}
		if len(key) > 0 {
			ret[key] = value
		}
	}

	return ret, nil
}

func translateKvpError(source error, illegalSuggestion string) error {
	j, ok := source.(*wmiext.JobError)

	if !ok {
		return source
	}

	var message string
	switch j.ErrorCode {
	case KvpOperationFailed:
		message = "Operation failed"
	case KvpAccessDenied:
		message = "Access denied"
	case KvpNotSupported:
		message = "Not supported"
	case KvpStatusUnknown:
		message = "Status is unknown"
	case KvpTimeoutOccurred:
		message = "Timeout occurred"
	case KvpIllegalArgument:
		message = "Illegal argument (" + illegalSuggestion + ")"
	case KvpSystemInUse:
		message = "System is in use"
	case KvpInvalidState:
		message = "Invalid state for this operation"
	case KvpIncorrectDataType:
		message = "Incorrect data type"
	case KvpSystemNotAvailable:
		message = "System is not available"
	case KvpOutOfMemory:
		message = "Out of memory"
	case KvpNotFound:
		message = "Not found"
	default:
		return source
	}

	return &KvpError{j.ErrorCode, message}
}
