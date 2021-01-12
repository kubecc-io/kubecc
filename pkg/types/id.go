package types

import (
	"crypto"
	"encoding/json"
)

type AgentID string

func (a *AgentInfo) AgentID() (AgentID, error) {
	bytes, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	sha1 := crypto.SHA256.New()
	sha1.Write(bytes)
	return AgentID(sha1.Sum(nil)), nil
}
