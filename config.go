// Copyright © 2023 Meroxa, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package discord

//go:generate paramgen -output=paramgen_src.go SourceConfig

import (
	"fmt"
	"net/http"
	"time"

	sdk "github.com/conduitio/conduit-connector-sdk"
)

type SourceConfig struct {
	// channel id
	ChannelID string `json:"channel-id" validate:"required"`
	// bot token
	Token string `json:"token" validate:"required"`
	// how often the connector will get data from the url
	PollingPeriod time.Duration `json:"pollingPeriod" default:"5m"`
	// todo
	Snapshot bool `default:"false"`

	// private field
	url string
}

const discordURL = "https://discord.com/api/v10/channels/%s/messages"

func (s SourceConfig) ParseConfig(cfg map[string]string) (SourceConfig, http.Header, error) {
	err := sdk.Util.ParseConfig(cfg, &s)
	if err != nil {
		return SourceConfig{}, nil, fmt.Errorf("invalid config: %w", err)
	}
	header := http.Header{}
	header.Add("Authorization", "Bot "+s.Token)
	s.url = fmt.Sprintf(discordURL, s.ChannelID)
	return s, header, nil
}
