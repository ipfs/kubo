package pinningservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

func NewRouter(authToken string, svc *PinningService) http.Handler {
	router := httprouter.New()
	router.GET("/api/v1/pins", svc.listPins)
	router.POST("/api/v1/pins", svc.addPin)
	router.GET("/api/v1/pins/:requestID", svc.getPin)
	router.POST("/api/v1/pins/:requestID", svc.replacePin)
	router.DELETE("/api/v1/pins/:requestID", svc.removePin)

	handler := authHandler(authToken, router)

	return handler
}

func authHandler(authToken string, delegate http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			errResp(w, "invalid authorization token, must start with 'Bearer '", "", http.StatusBadRequest)
			return
		}

		token := strings.TrimPrefix(authz, "Bearer ")
		if token != authToken {
			errResp(w, "access denied", "", http.StatusUnauthorized)
			return
		}

		delegate.ServeHTTP(w, r)
	})
}

func New() *PinningService {
	return &PinningService{
		PinAdded: func(*AddPinRequest, *PinStatus) {},
	}
}

// PinningService is a basic pinning service that implements the Remote Pinning API, for testing Kubo's integration with remote pinning services.
// Pins are not persisted, they are just kept in-memory, and this provides callbacks for controlling the behavior of the pinning service.
type PinningService struct {
	m sync.Mutex
	// PinAdded is a callback that is invoked after a new pin is added via the API.
	PinAdded func(*AddPinRequest, *PinStatus)
	pins     []*PinStatus
}

type Pin struct {
	CID     string                 `json:"cid"`
	Name    string                 `json:"name"`
	Origins []string               `json:"origins"`
	Meta    map[string]interface{} `json:"meta"`
}

type PinStatus struct {
	M         sync.Mutex
	RequestID string
	Status    string
	Created   time.Time
	Pin       Pin
	Delegates []string
	Info      map[string]interface{}
}

func (p *PinStatus) MarshalJSON() ([]byte, error) {
	type pinStatusJSON struct {
		RequestID string                 `json:"requestid"`
		Status    string                 `json:"status"`
		Created   time.Time              `json:"created"`
		Pin       Pin                    `json:"pin"`
		Delegates []string               `json:"delegates"`
		Info      map[string]interface{} `json:"info"`
	}
	// lock the pin before marshaling it to protect against data races while marshaling
	p.M.Lock()
	pinJSON := pinStatusJSON{
		RequestID: p.RequestID,
		Status:    p.Status,
		Created:   p.Created,
		Pin:       p.Pin,
		Delegates: p.Delegates,
		Info:      p.Info,
	}
	p.M.Unlock()
	return json.Marshal(pinJSON)
}

func (p *PinStatus) Clone() PinStatus {
	return PinStatus{
		RequestID: p.RequestID,
		Status:    p.Status,
		Created:   p.Created,
		Pin:       p.Pin,
		Delegates: p.Delegates,
		Info:      p.Info,
	}
}

const (
	matchExact    = "exact"
	matchIExact   = "iexact"
	matchPartial  = "partial"
	matchIPartial = "ipartial"

	statusQueued  = "queued"
	statusPinning = "pinning"
	statusPinned  = "pinned"
	statusFailed  = "failed"

	timeLayout = "2006-01-02T15:04:05.999Z"
)

func errResp(w http.ResponseWriter, reason, details string, statusCode int) {
	type errorObj struct {
		Reason  string `json:"reason"`
		Details string `json:"details"`
	}
	type errorResp struct {
		Error errorObj `json:"error"`
	}
	resp := errorResp{
		Error: errorObj{
			Reason:  reason,
			Details: details,
		},
	}
	writeJSON(w, resp, statusCode)
}

func writeJSON(w http.ResponseWriter, val any, statusCode int) {
	b, err := json.Marshal(val)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		errResp(w, fmt.Sprintf("marshaling response: %s", err), "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(b)
}

type AddPinRequest struct {
	CID     string                 `json:"cid"`
	Name    string                 `json:"name"`
	Origins []string               `json:"origins"`
	Meta    map[string]interface{} `json:"meta"`
}

func (p *PinningService) addPin(writer http.ResponseWriter, req *http.Request, params httprouter.Params) {
	var addReq AddPinRequest
	err := json.NewDecoder(req.Body).Decode(&addReq)
	if err != nil {
		errResp(writer, fmt.Sprintf("unmarshaling req: %s", err), "", http.StatusBadRequest)
		return
	}

	pin := &PinStatus{
		RequestID: uuid.NewString(),
		Status:    statusQueued,
		Created:   time.Now(),
		Pin:       Pin(addReq),
	}

	p.m.Lock()
	p.pins = append(p.pins, pin)
	p.m.Unlock()

	writeJSON(writer, &pin, http.StatusAccepted)
	p.PinAdded(&addReq, pin)
}

type ListPinsResponse struct {
	Count   int          `json:"count"`
	Results []*PinStatus `json:"results"`
}

func (p *PinningService) listPins(writer http.ResponseWriter, req *http.Request, params httprouter.Params) {
	q := req.URL.Query()

	cidStr := q.Get("cid")
	name := q.Get("name")
	match := q.Get("match")
	status := q.Get("status")
	beforeStr := q.Get("before")
	afterStr := q.Get("after")
	limitStr := q.Get("limit")
	metaStr := q.Get("meta")

	if limitStr == "" {
		limitStr = "10"
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		errResp(writer, fmt.Sprintf("parsing limit: %s", err), "", http.StatusBadRequest)
		return
	}

	var cids []string
	if cidStr != "" {
		cids = strings.Split(cidStr, ",")
	}

	var statuses []string
	if status != "" {
		statuses = strings.Split(status, ",")
	}

	p.m.Lock()
	defer p.m.Unlock()
	var pins []*PinStatus
	for _, pinStatus := range p.pins {
		// clone it so we can immediately release the lock
		pinStatus.M.Lock()
		clonedPS := pinStatus.Clone()
		pinStatus.M.Unlock()

		// cid
		var matchesCID bool
		if len(cids) == 0 {
			matchesCID = true
		} else {
			for _, cid := range cids {
				if cid == clonedPS.Pin.CID {
					matchesCID = true
				}
			}
		}
		if !matchesCID {
			continue
		}

		// name
		if match == "" {
			match = matchExact
		}
		if name != "" {
			switch match {
			case matchExact:
				if name != clonedPS.Pin.Name {
					continue
				}
			case matchIExact:
				if !strings.EqualFold(name, clonedPS.Pin.Name) {
					continue
				}
			case matchPartial:
				if !strings.Contains(clonedPS.Pin.Name, name) {
					continue
				}
			case matchIPartial:
				if !strings.Contains(strings.ToLower(clonedPS.Pin.Name), strings.ToLower(name)) {
					continue
				}
			default:
				errResp(writer, fmt.Sprintf("unknown match %q", match), "", http.StatusBadRequest)
				return
			}
		}

		// status
		var matchesStatus bool
		if len(statuses) == 0 {
			statuses = []string{statusPinned}
		}
		for _, status := range statuses {
			if status == clonedPS.Status {
				matchesStatus = true
			}
		}
		if !matchesStatus {
			continue
		}

		// before
		if beforeStr != "" {
			before, err := time.Parse(timeLayout, beforeStr)
			if err != nil {
				errResp(writer, fmt.Sprintf("parsing before: %s", err), "", http.StatusBadRequest)
				return
			}
			if !clonedPS.Created.Before(before) {
				continue
			}
		}

		// after
		if afterStr != "" {
			after, err := time.Parse(timeLayout, afterStr)
			if err != nil {
				errResp(writer, fmt.Sprintf("parsing before: %s", err), "", http.StatusBadRequest)
				return
			}
			if !clonedPS.Created.After(after) {
				continue
			}
		}

		// meta
		if metaStr != "" {
			meta := map[string]interface{}{}
			err := json.Unmarshal([]byte(metaStr), &meta)
			if err != nil {
				errResp(writer, fmt.Sprintf("parsing meta: %s", err), "", http.StatusBadRequest)
				return
			}
			var matchesMeta bool
			for k, v := range meta {
				pinV, contains := clonedPS.Pin.Meta[k]
				if !contains || !reflect.DeepEqual(pinV, v) {
					matchesMeta = false
					break
				}
			}
			if !matchesMeta {
				continue
			}
		}

		// add the original pin status, not the cloned one
		pins = append(pins, pinStatus)

		if len(pins) == limit {
			break
		}
	}

	out := ListPinsResponse{
		Count:   len(pins),
		Results: pins,
	}
	writeJSON(writer, out, http.StatusOK)
}

func (p *PinningService) getPin(writer http.ResponseWriter, req *http.Request, params httprouter.Params) {
	requestID := params.ByName("requestID")
	p.m.Lock()
	defer p.m.Unlock()
	for _, pin := range p.pins {
		if pin.RequestID == requestID {
			writeJSON(writer, pin, http.StatusOK)
			return
		}
	}
	errResp(writer, "", "", http.StatusNotFound)
}

func (p *PinningService) replacePin(writer http.ResponseWriter, req *http.Request, params httprouter.Params) {
	requestID := params.ByName("requestID")

	var replaceReq Pin
	err := json.NewDecoder(req.Body).Decode(&replaceReq)
	if err != nil {
		errResp(writer, fmt.Sprintf("decoding request: %s", err), "", http.StatusBadRequest)
		return
	}

	p.m.Lock()
	defer p.m.Unlock()
	for _, pin := range p.pins {
		if pin.RequestID == requestID {
			pin.M.Lock()
			pin.Pin = replaceReq
			pin.M.Unlock()
			writer.WriteHeader(http.StatusAccepted)
			return
		}
	}
	errResp(writer, "", "", http.StatusNotFound)
}

func (p *PinningService) removePin(writer http.ResponseWriter, req *http.Request, params httprouter.Params) {
	requestID := params.ByName("requestID")

	p.m.Lock()
	defer p.m.Unlock()

	for i, pin := range p.pins {
		if pin.RequestID == requestID {
			p.pins = append(p.pins[0:i], p.pins[i+1:]...)
			writer.WriteHeader(http.StatusAccepted)
			return
		}
	}

	errResp(writer, "", "", http.StatusNotFound)
}
