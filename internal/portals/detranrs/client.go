package detranrs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/paulorosantos/desphub-rpa/internal/model"
)

var ErrSessaoExpirada = errors.New("detranrs: sessão inválida/expirada")

const (
	DefaultBaseURL = "https://pcsdetran.procergs.com.br"
	spaOrigin  = "https://pcsdetran.rs.gov.br"
	spaReferer = "https://pcsdetran.rs.gov.br/"
)

type Auth struct {
	Bearer string // Authorization: Bearer <token>
	UserID string // X-User-Id (base64 do CPF do despachante)
}

type Client struct {
	http    *http.Client
	baseURL string
	auth    Auth
	ua      string
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
		ua:      "Mozilla/5.0 (compatible; DespHubRPA/1.0)",
	}
}

func (c *Client) SetAuth(a Auth) { c.auth = a }

func (c *Client) SetUserAgent(ua string) {
	if ua != "" {
		c.ua = ua
	}
}

func (c *Client) Nome() string { return "DETRAN-RS" }

func (c *Client) Consultar(ctx context.Context, placa, renavam string) (*model.ConsultaResponse, error) {
	raw, err := c.ConsultarVeiculo(ctx, placa, renavam)
	if err != nil {
		return nil, err
	}
	return ParseVeiculo(raw)
}

func (c *Client) ConsultarVeiculo(ctx context.Context, placa, renavam string) ([]byte, error) {
	if c.auth.Bearer == "" || c.auth.UserID == "" {
		return nil, fmt.Errorf("detranrs: credenciais ausentes (Bearer/X-User-Id) — faça o login primeiro")
	}

	endpoint := fmt.Sprintf("%s/pcsdetran/rest/veiculos/%s/", c.baseURL, url.PathEscape(placa))
	q := url.Values{}
	q.Set("renavam", renavam)
	q.Set("contabiliza", "false")
	endpoint += "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Authorization", "Bearer "+c.auth.Bearer)
	req.Header.Set("X-User-Id", c.auth.UserID)
	req.Header.Set("Origin", spaOrigin)
	req.Header.Set("Referer", spaReferer)
	req.Header.Set("User-Agent", c.ua)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("detranrs: request veículo: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("detranrs: lendo corpo: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w (HTTP %d)", ErrSessaoExpirada, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("detranrs: API devolveu HTTP %d", resp.StatusCode)
	}
	return body, nil
}
