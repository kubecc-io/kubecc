package types

import (
	"crypto"
	"encoding/json"
	"errors"
)

type AgentID string

func (a *AgentInfo) AgentID() (AgentID, error) {
	if a == nil {
		return "", errors.New("AgentID is nil")
	}
	bytes, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	sha1 := crypto.SHA256.New()
	_, err = sha1.Write(bytes)
	if err != nil {
		return "", err
	}
	return AgentID(sha1.Sum(nil)), nil
}
