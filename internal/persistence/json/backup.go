package json

import "encoding/json"

func ValidateBackup(raw []byte) error {
	var value dataModel
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	return checkData(value)
}
