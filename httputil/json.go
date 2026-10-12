package httputil

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
)

var durationJSONMarshalers = json.MarshalFunc(func(value time.Duration) ([]byte, error) {
	return strconv.AppendInt(nil, int64(value), 10), nil
})

var durationJSONUnmarshalers = json.UnmarshalFunc(func(data []byte, value *time.Duration) error {
	var nanoseconds int64
	if err := json.Unmarshal(data, &nanoseconds); err != nil {
		return err
	}
	*value = time.Duration(nanoseconds)
	return nil
})

func marshalJSON(value any) ([]byte, error) {
	return json.Marshal(value,
		json.Deterministic(true),
		json.FormatNilMapAsNull(true),
		json.FormatNilSliceAsNull(true),
		jsontext.EscapeForHTML(true),
		jsontext.EscapeForJS(true),
		json.WithMarshalers(durationJSONMarshalers),
	)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	body, err := marshalJSON(data)
	if err != nil {
		writeInternalJSONError(w)
		return
	}

	w.Header().Set("Content-Type", mimeApplicationJSON)
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

func writeInternalJSONError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", mimeApplicationJSON)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(`{"code":"` + httperr.CodeInternalError + `","message":"` + msgInternalServerError + `"}` + "\n"))
}
