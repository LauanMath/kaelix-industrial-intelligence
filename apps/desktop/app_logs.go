package main

import (
	"errors"
	c "kaelix/contracts"
	"net/url"
	"regexp"
	"strconv"
)

var gatewayUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (a *App) GetGatewayLogs(gateway, cursor string) (c.GatewayLogPage, error) {
	out := c.GatewayLogPage{Items: []c.StoredGatewayLog{}}
	if !gatewayUUID.MatchString(gateway) {
		return out, errors.New("selecione um gateway válido")
	}
	if cursor != "" {
		n, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || n < 0 {
			return out, errors.New("cursor de diagnóstico inválido")
		}
	}
	cl, ctx, err := a.remote()
	if err != nil {
		return out, err
	}
	q := url.Values{"limit": {"200"}}
	if cursor != "" {
		q.Set("after_id", cursor)
	}
	err = cl.Request(ctx, "GET", "/api/v1/gateways/"+url.PathEscape(gateway)+"/logs?"+q.Encode(), nil, &out)
	return out, a.apiFailure(cl, err)
}
