package jsplugin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Calcium-Ion/moejs"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type volcSignRequest struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	AccessKey string            `json:"accessKey"`
	SecretKey string            `json:"secretKey"`
	Region    string            `json:"region"`
	Service   string            `json:"service"`
	Timestamp int64             `json:"timestamp"`
}

type tencentSignRequest struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	SecretId  string            `json:"secretId"`
	SecretKey string            `json:"secretKey"`
	Service   string            `json:"service"`
	Timestamp int64             `json:"timestamp"`
}

// injectGlobals installs utils and console. currentCall returns the hook call
// running on runtime, or nil outside one; utils.fetch reads its fetcher and
// budget there, so no request data lives in the runtime's globals.
func injectGlobals(runtime *moejs.Runtime, identity func() string, now func() time.Time, logOutput func(string), currentCall func() *runtimeCall) error {
	utils := map[string]any{
		"fetch": runtime.Function("fetch", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			call := currentCall()
			if call == nil {
				return moejs.Undefined(), fmt.Errorf("utils.fetch is not available outside hooks")
			}
			return call.fetch(r, moejs.Arg(args, 0))
		}),
		"hasCapability": runtime.Function("hasCapability", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			name, err := stringValue(r, moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			return moejs.Bool(HasCapability(name)), nil
		}),
		"json": map[string]any{
			"clone": runtime.Function("clone", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
				if moejs.Arg(args, 0).IsUndefined() {
					return moejs.Undefined(), r.TypeError("json.clone requires a JSON value")
				}
				budget := &jsonStateBudget{ctx: context.Background(), bytes: MaxJSONToolBytes, nodes: maxJSONToolNodes, realm: r}
				value, err := newJSONStateNode(moejs.Arg(args, 0), 0, budget)
				if err != nil {
					var exception *moejs.Exception
					var interrupted *moejs.InterruptedError
					if errors.As(err, &exception) || errors.As(err, &interrupted) {
						return moejs.Undefined(), err
					}
					return moejs.Undefined(), r.TypeError("json.clone: %s", err)
				}
				return value.jsValue(r)
			}),
		},
		"unixNow": runtime.Function("unixNow", 0, func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
			return moejs.Int(now().Unix()), nil
		}),
		"jwtSignHS256": runtime.Function("jwtSignHS256", 2, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			var claims map[string]any
			if argument := moejs.Arg(args, 0); !argument.IsNullish() {
				exported, err := r.ToGoStrict(argument)
				if err != nil {
					return moejs.Undefined(), err
				}
				var ok bool
				if claims, ok = exported.(map[string]any); !ok {
					return moejs.Undefined(), r.TypeError("jwtSignHS256 claims must be an object")
				}
			}
			secret, err := stringValue(r, moejs.Arg(args, 1))
			if err != nil {
				return moejs.Undefined(), err
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims)).SignedString([]byte(secret))
			if err != nil {
				return moejs.Undefined(), err
			}
			return moejs.String(token), nil
		}),
		"hmacSHA256": runtime.Function("hmacSHA256", 2, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			message, err := stringValue(r, moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			secret, err := stringValue(r, moejs.Arg(args, 1))
			if err != nil {
				return moejs.Undefined(), err
			}
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write([]byte(message))
			return moejs.String(hex.EncodeToString(mac.Sum(nil))), nil
		}),
		"base64": runtime.Function("base64", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			value, err := stringValue(r, moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			return moejs.String(base64.StdEncoding.EncodeToString([]byte(value))), nil
		}),
		"base64URL": runtime.Function("base64URL", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			value, err := stringValue(r, moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			return moejs.String(base64.RawURLEncoding.EncodeToString([]byte(value))), nil
		}),
		"base64URLDecode": runtime.Function("base64URLDecode", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			value, err := stringValue(r, moejs.Arg(args, 0))
			if err != nil {
				return moejs.Undefined(), err
			}
			decoded, err := base64.RawURLEncoding.DecodeString(value)
			if err != nil {
				return moejs.Undefined(), err
			}
			return moejs.String(string(decoded)), nil
		}),
		"uuid": runtime.Function("uuid", 0, func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
			return moejs.String(uuid.NewString()), nil
		}),
		"volcSignV4": runtime.Function("volcSignV4", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			// Fields are read by their Go names, as plugins have always
			// passed them.
			var request volcSignRequest
			argument := moejs.Arg(args, 0)
			if !argument.IsNullish() && !argument.IsObject() {
				return moejs.Undefined(), r.TypeError("volcSignV4 request must be an object")
			}
			if argument.IsObject() {
				object := argument.AsObject()
				fields := []struct {
					name   string
					target *string
				}{
					{"Method", &request.Method}, {"URL", &request.URL}, {"Body", &request.Body},
					{"AccessKey", &request.AccessKey}, {"SecretKey", &request.SecretKey},
					{"Region", &request.Region}, {"Service", &request.Service},
				}
				for _, field := range fields {
					value, err := object.GetProp(r, r.KeyFromGoString(field.name))
					if err != nil {
						return moejs.Undefined(), err
					}
					if *field.target, err = stringValue(r, value); err != nil {
						return moejs.Undefined(), err
					}
				}
				headers, err := object.GetProp(r, r.KeyFromGoString("Headers"))
				if err != nil {
					return moejs.Undefined(), err
				}
				if headers.IsObject() {
					keys, err := r.EnumerableOwnKeys(headers.AsObject())
					if err != nil {
						return moejs.Undefined(), err
					}
					request.Headers = make(map[string]string, len(keys))
					for _, key := range keys {
						value, err := headers.AsObject().GetProp(r, key)
						if err != nil {
							return moejs.Undefined(), err
						}
						if request.Headers[key.GoString()], err = stringValue(r, value); err != nil {
							return moejs.Undefined(), err
						}
					}
				}
				timestamp, err := object.GetProp(r, r.KeyFromGoString("Timestamp"))
				if err != nil {
					return moejs.Undefined(), err
				}
				if !timestamp.IsNullish() {
					seconds, err := r.ToNumber(timestamp)
					if err != nil {
						return moejs.Undefined(), err
					}
					if !math.IsNaN(seconds) && !math.IsInf(seconds, 0) {
						request.Timestamp = int64(seconds)
					}
				}
			}
			signed, err := signVolcV4(request, now)
			if err != nil {
				return moejs.Undefined(), err
			}
			return r.FromGo(signed)
		}),
		"tencentSignTC3": runtime.Function("tencentSignTC3", 1, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			var request tencentSignRequest
			argument := moejs.Arg(args, 0)
			if !argument.IsNullish() && !argument.IsObject() {
				return moejs.Undefined(), r.TypeError("tencentSignTC3 request must be an object")
			}
			if argument.IsObject() {
				object := argument.AsObject()
				fields := []struct {
					name   string
					target *string
				}{
					{"Method", &request.Method}, {"URL", &request.URL}, {"Body", &request.Body},
					{"SecretId", &request.SecretId}, {"SecretKey", &request.SecretKey},
					{"Service", &request.Service},
				}
				for _, field := range fields {
					value, err := object.GetProp(r, r.KeyFromGoString(field.name))
					if err != nil {
						return moejs.Undefined(), err
					}
					if *field.target, err = stringValue(r, value); err != nil {
						return moejs.Undefined(), err
					}
				}
				headers, err := object.GetProp(r, r.KeyFromGoString("Headers"))
				if err != nil {
					return moejs.Undefined(), err
				}
				if headers.IsObject() {
					keys, err := r.EnumerableOwnKeys(headers.AsObject())
					if err != nil {
						return moejs.Undefined(), err
					}
					request.Headers = make(map[string]string, len(keys))
					for _, key := range keys {
						value, err := headers.AsObject().GetProp(r, key)
						if err != nil {
							return moejs.Undefined(), err
						}
						if request.Headers[key.GoString()], err = stringValue(r, value); err != nil {
							return moejs.Undefined(), err
						}
					}
				}
				timestamp, err := object.GetProp(r, r.KeyFromGoString("Timestamp"))
				if err != nil {
					return moejs.Undefined(), err
				}
				if !timestamp.IsNullish() {
					seconds, err := r.ToNumber(timestamp)
					if err != nil {
						return moejs.Undefined(), err
					}
					if !math.IsNaN(seconds) && !math.IsInf(seconds, 0) {
						request.Timestamp = int64(seconds)
					}
				}
			}
			signed, err := signTencentTC3(request, now)
			if err != nil {
				return moejs.Undefined(), err
			}
			return r.FromGo(signed)
		}),
	}
	if err := runtime.SetGlobal("utils", utils); err != nil {
		return err
	}
	return runtime.SetGlobal("console", map[string]any{
		"log": runtime.Function("log", 0, func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			parts := make([]string, len(args))
			for i, argument := range args {
				if argument.IsSymbol() {
					parts[i] = argument.String()
					continue
				}
				text, err := r.ToString(argument)
				if err != nil {
					return moejs.Undefined(), err
				}
				parts[i] = text.GoString()
			}
			if logOutput != nil {
				logOutput(identity() + " " + strings.Join(parts, " "))
			}
			return moejs.Undefined(), nil
		}),
	})
}

// stringValue reads a host function's string parameter: undefined and null
// read as "", other values as JavaScript's String(value).
func stringValue(r *moejs.Realm, value moejs.Value) (string, error) {
	if value.IsNullish() {
		return "", nil
	}
	if value.IsString() {
		return value.AsString().GoString(), nil
	}
	text, err := r.ToString(value)
	if err != nil {
		return "", err
	}
	return text.GoString(), nil
}

func signVolcV4(request volcSignRequest, now func() time.Time) (map[string]string, error) {
	parsedURL, err := url.Parse(request.URL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid Volcengine signing URL")
	}
	region := request.Region
	if region == "" {
		region = "cn-north-1"
	}
	service := request.Service
	if service == "" {
		service = "cv"
	}
	timestamp := now().UTC()
	if request.Timestamp != 0 {
		timestamp = time.Unix(request.Timestamp, 0).UTC()
	}
	xDate := timestamp.Format("20060102T150405Z")
	shortDate := timestamp.Format("20060102")
	bodyHash := sha256.Sum256([]byte(request.Body))
	requestPath := parsedURL.EscapedPath()
	if requestPath == "" {
		requestPath = "/"
	}

	headers := make(map[string]string, len(request.Headers)+3)
	for name, value := range request.Headers {
		headers[strings.ToLower(name)] = strings.TrimSpace(value)
	}
	headers["host"] = parsedURL.Host
	headers["x-date"] = xDate
	headers["x-content-sha256"] = hex.EncodeToString(bodyHash[:])
	keys := make([]string, 0, len(headers))
	for name := range headers {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	var canonicalHeaders strings.Builder
	for _, name := range keys {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(headers[name])
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(keys, ";")
	canonicalRequest := strings.Join([]string{
		strings.ToUpper(request.Method), requestPath, parsedURL.Query().Encode(),
		canonicalHeaders.String(), signedHeaders, hex.EncodeToString(bodyHash[:]),
	}, "\n")
	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	scope := fmt.Sprintf("%s/%s/%s/request", shortDate, region, service)
	stringToSign := fmt.Sprintf("HMAC-SHA256\n%s\n%s\n%s", xDate, scope, hex.EncodeToString(canonicalHash[:]))
	sign := func(key []byte, value string) []byte {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(value))
		return mac.Sum(nil)
	}
	signingKey := sign(sign(sign([]byte(request.SecretKey), shortDate), region), service)
	signingKey = sign(signingKey, "request")
	signature := hex.EncodeToString(sign(signingKey, stringToSign))
	return map[string]string{
		"Authorization":    fmt.Sprintf("HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", request.AccessKey, scope, signedHeaders, signature),
		"X-Date":           xDate,
		"X-Content-Sha256": hex.EncodeToString(bodyHash[:]),
	}, nil
}

func signTencentTC3(request tencentSignRequest, now func() time.Time) (map[string]string, error) {
	parsedURL, err := url.Parse(request.URL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid Tencent Cloud signing URL")
	}
	if request.SecretId == "" || request.SecretKey == "" {
		return nil, fmt.Errorf("Tencent Cloud SecretId and SecretKey are required")
	}
	service := request.Service
	if service == "" {
		host, _, _ := strings.Cut(parsedURL.Hostname(), ".")
		service = host
	}
	if service == "" {
		return nil, fmt.Errorf("Tencent Cloud service is required")
	}
	timestamp := now().UTC()
	if request.Timestamp != 0 {
		timestamp = time.Unix(request.Timestamp, 0).UTC()
	}
	unixTime := timestamp.Unix()
	date := timestamp.Format("2006-01-02")
	bodyHash := sha256.Sum256([]byte(request.Body))
	payloadHash := hex.EncodeToString(bodyHash[:])

	headers := map[string]string{
		"content-type": "application/json",
		"host":         parsedURL.Host,
	}
	for name, value := range request.Headers {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || key == "authorization" {
			continue
		}
		headers[key] = strings.TrimSpace(value)
	}
	headers["host"] = parsedURL.Host
	keys := make([]string, 0, len(headers))
	for name := range headers {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	// content-type keeps its original case and spacing; other signed
	// headers are lowercased per the TC3 canonical-header rule.
	var canonicalHeaders strings.Builder
	for _, name := range keys {
		value := headers[name]
		if name != "content-type" {
			value = strings.ToLower(value)
		}
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(value)
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(keys, ";")
	canonicalRequest := strings.Join([]string{
		strings.ToUpper(request.Method), "/", "",
		canonicalHeaders.String(), signedHeaders, payloadHash,
	}, "\n")
	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	scope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s\n%s", unixTime, scope, hex.EncodeToString(canonicalHash[:]))
	sign := func(key []byte, value string) []byte {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(value))
		return mac.Sum(nil)
	}
	secretDate := sign([]byte("TC3"+request.SecretKey), date)
	secretService := sign(secretDate, service)
	secretSigning := sign(secretService, "tc3_request")
	signature := hex.EncodeToString(sign(secretSigning, stringToSign))
	return map[string]string{
		"Authorization":  fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", request.SecretId, scope, signedHeaders, signature),
		"X-TC-Timestamp": fmt.Sprintf("%d", unixTime),
		"Content-Type":   headers["content-type"],
		"Host":           parsedURL.Host,
	}, nil
}
