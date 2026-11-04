package types

import (
	"encoding/json"
	"errors"
	"time"
)

type ConfigSpec struct {
	Name   string
	Driver ConfigDriverSpec
	Labels map[string]string
}

type ConfigVersion struct {
	Index int
}

type ConfigDriverSpec struct {
	Name    string
	Options map[string]string
}

type ConfigCreateReport struct {
	ID string
}

type ConfigListReport struct {
	ID        string
	Name      string
	Driver    string
	CreatedAt string
	UpdatedAt string
}

type ConfigRmReport struct {
	ID  string `json:"ID,omitempty"`
	Err error  `json:"Err,omitempty"`
}

type configRmReportHelper struct {
	ID  string `json:"ID,omitempty"`
	Err string `json:"Err,omitempty"`
}

func (r ConfigRmReport) MarshalJSON() ([]byte, error) {
	helper := configRmReportHelper{ID: r.ID}
	if r.Err != nil {
		helper.Err = r.Err.Error()
	}
	return json.Marshal(helper)
}

func (r *ConfigRmReport) UnmarshalJSON(data []byte) error {
	var helper configRmReportHelper
	if err := json.Unmarshal(data, &helper); err != nil {
		return err
	}
	r.ID = helper.ID
	if helper.Err == "" {
		r.Err = nil
		return nil
	}
	r.Err = errors.New(helper.Err)
	return nil
}

type ConfigInfoReport struct {
	ID         string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Spec       ConfigSpec
	ConfigData string `json:"ConfigData,omitempty"`
}

type ConfigInfoReportCompat struct {
	ConfigInfoReport
	Version ConfigVersion
}
