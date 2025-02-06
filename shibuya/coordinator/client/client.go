package client

// This package is used both by coordinator and engines
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rakutentech/shibuya/shibuya/coordinator/api"
	enginesModel "github.com/rakutentech/shibuya/shibuya/engines/model"
	"github.com/rakutentech/shibuya/shibuya/model"
)

type Client struct {
	httpClient *http.Client
}

var (
	CollectionTriggerError = errors.New("Collection trigger error")
	CollectionTermError    = errors.New("Collection term error")
	PlanTermError          = errors.New("Plan term error")
)

type ReqOpts struct {
	Endpoint string
	APIKey   string
}

func NewClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

func (c *Client) TriggerCollection(ro ReqOpts, collection *model.Collection,
	dataConfig map[int64]enginesModel.PlanEnginesConfig, plans []*model.Plan) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := prepareEngineData(writer, dataConfig); err != nil {
		return err
	}
	if err := preparePlanFiles(writer, plans, collection); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	url := c.makeUrl(ro.Endpoint, collection.ID)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := c.sendRequest(req, ro); err != nil {
		return fmt.Errorf("%w:%w", CollectionTriggerError, err)
	}
	return nil
}

func (c *Client) Healthcheck(ro ReqOpts, collection *model.Collection, numberOfEngines int) error {
	resourceUrl := c.makeUrl(ro.Endpoint, collection.ID)
	values := url.Values{}
	values.Add("engines", strconv.Itoa(numberOfEngines))
	req, err := c.makeRequestWithValues(resourceUrl, http.MethodGet, values)
	if err != nil {
		return err
	}
	return c.sendRequest(req, ro)
}

func (c *Client) ProgressCheck(ro ReqOpts, collectionID int64, planID int64) error {
	endpoint := c.makeUrl(ro.Endpoint, collectionID)
	resourceUrl := fmt.Sprintf("%s/%d", endpoint, planID)
	req, err := http.NewRequest("GET", resourceUrl, nil)
	if err != nil {
		return err
	}
	return c.sendRequest(req, ro)
}

func (c *Client) ReportProgress(ro ReqOpts, collectionID, planID string, engineID int, status bool) error {
	cid, err := strconv.ParseInt(collectionID, 10, 64)
	if err != nil {
		return err
	}
	endpoint := c.makeUrl(ro.Endpoint, cid)
	values := url.Values{}
	values.Add("running", strconv.FormatBool(status))
	resourceUrl := fmt.Sprintf("%s/%s/%d", endpoint, planID, engineID)
	req, err := c.makeRequestWithValues(resourceUrl, http.MethodPut, values)
	if err != nil {
		return err
	}
	return c.sendRequest(req, ro)
}

func (c *Client) TermCollection(ro ReqOpts, collectionID int64, eps []*model.ExecutionPlan) error {
	endpoint := c.makeUrl(ro.Endpoint, collectionID)
	planIDs := make([]string, len(eps))
	for i, ep := range eps {
		planIDs[i] = strconv.Itoa(int(ep.PlanID))
	}
	values := url.Values{}
	values.Add("plans", strings.Join(planIDs, ","))
	req, err := c.makeRequestWithValues(endpoint, http.MethodDelete, values)
	if err != nil {
		return err
	}
	if err := c.sendRequest(req, ro); err != nil {
		return fmt.Errorf("%w:%w", CollectionTermError, err)
	}
	return nil
}

func (c *Client) TermPlan(ro ReqOpts, collectionID, planID int64) error {
	endpoint := c.makeUrl(ro.Endpoint, collectionID)
	resourceUrl := fmt.Sprintf("%s/%d", endpoint, planID)
	req, err := http.NewRequest("DELETE", resourceUrl, nil)
	if err != nil {
		return err
	}
	if err := c.sendRequest(req, ro); err != nil {
		return fmt.Errorf("%w:%w", PlanTermError, err)
	}
	return nil
}

func (c *Client) FetchFile(ro ReqOpts, path string) ([]byte, error) {
	resourceUrl := fmt.Sprintf("https://%s/%s", ro.Endpoint, path)
	req, err := http.NewRequest("GET", resourceUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bear %s", ro.APIKey))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) makeRequestWithValues(resourceUrl, method string, values url.Values) (*http.Request, error) {
	if values == nil {
		return http.NewRequest(resourceUrl, method, nil)
	}
	switch method {
	case http.MethodGet, http.MethodDelete:
		req, err := http.NewRequest(method, resourceUrl, nil)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = values.Encode()
		return req, nil
	case http.MethodPut, http.MethodPost:
		req, err := http.NewRequest(method, resourceUrl, strings.NewReader(values.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	}
	return nil, nil
}

func (c *Client) sendRequest(req *http.Request, ro ReqOpts) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bear %s", ro.APIKey))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	return handleResponse(resp)
}

func (c *Client) makeUrl(endpoint string, collectionID int64) string {
	if strings.Contains(endpoint, "http") {
		return fmt.Sprintf("%s/api/collections/%d", endpoint, collectionID)
	}
	return fmt.Sprintf("https://%s/api/collections/%d", endpoint, collectionID)
}

// TODO: shall we move this to a lib so it can share with Shibuya api client as well?
func handleResponse(resp *http.Response) error {
	defer resp.Body.Close()
	switch status := resp.StatusCode; {
	case status >= 400:
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("resp: %s, status_code: %d", string(raw), status)
	case status == 200:
		return nil
	}
	return nil
}

func prepareEngineData(writer *multipart.Writer, dataConfig map[int64]enginesModel.PlanEnginesConfig) error {
	payload, err := json.Marshal(dataConfig)
	if err != nil {
		return nil
	}
	// Add the JSON payload to the form data
	engineData, err := writer.CreateFormField("engine_data")
	if err != nil {
		return fmt.Errorf("could not create form field for JSON: %v", err)
	}
	_, err = engineData.Write(payload)
	if err != nil {
		return fmt.Errorf("could not write JSON to form field: %v", err)
	}
	return nil
}

func preparePlanFiles(writer *multipart.Writer, plans []*model.Plan, collection *model.Collection) error {
	for _, file := range collection.Data {
		ffk := api.FormFileKey(file.Filename)
		part, err := writer.CreateFormFile(ffk.MakeCollectionDataKey(), file.Filename)
		if err != nil {
			return err
		}
		// Copy the file content into the part
		if _, err := part.Write(file.Content); err != nil {
			return err
		}
	}
	for _, p := range plans {
		for _, file := range p.Data {
			ffk := api.FormFileKey(strconv.Itoa(int(p.ID)))
			part, err := writer.CreateFormFile(ffk.MakePlanDataKey(), file.Filename)
			if err != nil {
				return err
			}
			// Copy the file content into the part
			if _, err := part.Write(file.Content); err != nil {
				return err
			}
		}
		ffk := api.FormFileKey(strconv.Itoa(int(p.ID)))
		part, err := writer.CreateFormFile(ffk.MakeTestFileKey(), p.TestFile.Filename)
		if err != nil {
			return nil
		}
		if _, err := part.Write(p.TestFile.Content); err != nil {
			return nil
		}
	}
	return nil
}
