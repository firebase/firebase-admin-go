package auth

import (
	"crypto/rsa"
	"crypto/tls"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	kidFound    = "kidFound"
	kidNotFound = "kidNotFound"
)

var (
	fakeKey        *rsa.PublicKey
	fakeKeyHandler = func(w http.ResponseWriter, _ *http.Request) {
		header := w.Header()
		header.Set("Content-Type", "application/json")
		header.Set("Expires", timeNow().Add(time.Minute).Format(time.RFC1123))
		io.Copy(w, strings.NewReader(`
{
 "kidFound": "-----BEGIN CERTIFICATE-----\nMIIDHDCCAgSgAwIBAgIIG+vx2nTw//YwDQYJKoZIhvcNAQEFBQAwMTEvMC0GA1UE\nAxMmc2VjdXJldG9rZW4uc3lzdGVtLmdzZXJ2aWNlYWNjb3VudC5jb20wHhcNMTcw\nNTI1MDA0NTI2WhcNMTcwNTI4MDExNTI2WjAxMS8wLQYDVQQDEyZzZWN1cmV0b2tl\nbi5zeXN0ZW0uZ3NlcnZpY2VhY2NvdW50LmNvbTCCASIwDQYJKoZIhvcNAQEBBQAD\nggEPADCCAQoCggEBANWlry6lHzhV0QrcorrKprth5as8k+D8aCwiKb2XfBqMIJxN\npUBX7MngvHXmSxkjNS2hdfoOoRjuK8r8FiVVA9hdmM2Z44S3/iG26E/WWQP/Ti0z\nsFNUunjF2Cm5LiPUU20FUlIDDnzoTqVA06YAcEuqjy5uFf6yn+OurQaCXXIPeEgQ\nJkRoElzGpmt3iH/8AJ+jL/tH2q433GLZDJwT5GGOeStaiPYeaCQ/teLUjsgJ/EvF\njhxNofBmomIcyd3/6J7zmf9MgMVM+hVT8P88TXnqvc/76/NCZs+stfPiqNA4pb3S\nut86fOcR6JdvcOz2r45hq3T7q6pq8hFe5PX4QkECAwEAAaM4MDYwDAYDVR0TAQH/\nBAIwADAOBgNVHQ8BAf8EBAMCB4AwFgYDVR0lAQH/BAwwCgYIKwYBBQUHAwIwDQYJ\nKoZIhvcNAQEFBQADggEBAKO50BKeqG9j3f94FhvHEphoK2DANb1M3scvn/F+pQFZ\nc3mKpqCPaMamwcqsCFm2mDBN7kO3LchQKc0lYipfat2eBcf583xEANFCnqDMbpCq\nX8GWtIFvksvMcC6ZGfyt0HNAUcloQx7swibuGRRMNDPgSmyk8EkkG3IzdVPaT83h\ns4NCf3NVzMaBMiJcMnLQkmEdgPcRzj1ZzFcl5BPeFcgx+hAyjVF5NjY0fHJyBWD1\npzyxo4kuqJi/zUTxzUwaojBJp6Se/7+go+m/scy9JR/VQk7fVidHOgIW9PuBeVB/\n3IM1jC5xClIpHQPczut2g8lXthriZqrP5SWIaA3Vpw4=\n-----END CERTIFICATE-----\n",
 "otherKey1": "-----BEGIN CERTIFICATE-----\nMIIDHDCCAgSgAwIBAgIIen5/qqp1EXYwDQYJKoZIhvcNAQEFBQAwMTEvMC0GA1UE\nAxMmc2VjdXJldG9rZW4uc3lzdGVtLmdzZXJ2aWNlYWNjb3VudC5jb20wHhcNMTcw\nNTIzMDA0NTI2WhcNMTcwNTI2MDExNTI2WjAxMS8wLQYDVQQDEyZzZWN1cmV0b2tl\nbi5zeXN0ZW0uZ3NlcnZpY2VhY2NvdW50LmNvbTCCASIwDQYJKoZIhvcNAQEBBQAD\nggEPADCCAQoCggEBALO3nJg6fnbIyj7wC+SCztbqu4ntekDZrCKsbLSgXUPoPVbJ\nbJt5+/zPayb7iK/aW8uaGtk/o8VLEtQ/bThl2cK+NtjMiIMmQ/9FhKJs03YnjVgE\n/PQcBF8ZMvl1wMGasjJvE0EewgGBMaN5AYwYpZ3O1IDdr+oyo9U39ViFGibx8DMz\ny8RO4xirFQoYuuz7GU/0dSrk0XKnn/Z8jSgWKaWPuf5HQCpk+rvz8mhNJwfFt5Tq\n4W4ugWtVjBSo5ASeNYwVMadFgY63aLfGJIkQOJBAUdVTIe90p18WL0YGID12ZnBT\nduf2g2Lb/bPA6F4KlzUB4DadRbVLxgUIrCzEXK8CAwEAAaM4MDYwDAYDVR0TAQH/\nBAIwADAOBgNVHQ8BAf8EBAMCB4AwFgYDVR0lAQH/BAwwCgYIKwYBBQUHAwIwDQYJ\nKoZIhvcNAQEFBQADggEBACR9vnDu5LbWiJ3ltAsc0gMKylipQEuwD1byCtr1YmKD\nc8D35pKR3tVexL3yfnH6yp+g1Zv7T3TIghMTV6P9EWSpwh+e4a+8bXivfMYys/Bk\n/aWj11e9+aT5s2Ht2vGBSY1PpVDdP80w9ehHsL1xGRRV1haYdFXculN3Lj093qm1\nIDXL+B+tsJOThlMskxc4NXiav/q0C46BlXxDaqBN3kwtik990b2AKP2DtmIACbXe\nSR3qcDt5o5RU3BFPVgvb2SnMFWxfzSVQWRXJzrc8JNo+73x0wEFvh+erXDGLPRzE\nU1FR3h7FSdeEB5Kh87RjVqZP67j74XBHw30fLJ3OBvs=\n-----END CERTIFICATE-----\n",
 "otherKey2": "-----BEGIN CERTIFICATE-----\nMIIDHDCCAgSgAwIBAgIII3eNGNfSNl8wDQYJKoZIhvcNAQEFBQAwMTEvMC0GA1UE\nAxMmc2VjdXJldG9rZW4uc3lzdGVtLmdzZXJ2aWNlYWNjb3VudC5jb20wHhcNMTcw\nNTI0MDA0NTI2WhcNMTcwNTI3MDExNTI2WjAxMS8wLQYDVQQDEyZzZWN1cmV0b2tl\nbi5zeXN0ZW0uZ3NlcnZpY2VhY2NvdW50LmNvbTCCASIwDQYJKoZIhvcNAQEBBQAD\nggEPADCCAQoCggEBAOvF2rNTHyqChrIsNYNB/B5O8aK8mP+lM0kwwc3TUeIwO8Op\nWcZDsTN0k4VuPXbuFKwgvNuSBKlPXl/mDCKDnXRVONKAIjKXpTajE4r+Mqu9BIef\n/RUx13udon8YGxxcKrGjMZkuXQveUrFxmYy6SwCoo8i9F0vtnEtpLk+Z0Q2fdxwe\ncn5v8OCULm6ZNcRVyzsI9Qu9ogDiCPf/470oM1cW+VNFgt9V3On4USsRdUbSwxFc\nBxy0OMd9/FI8cRyMVpp9QayD7NjudieZxuveXuWm+L1dYyIGSBFIPNCv88nnywEj\n4wniTCqq68SAXdOm7VuQ9ciPq2gyRdwkocgj4PkCAwEAAaM4MDYwDAYDVR0TAQH/\nBAIwADAOBgNVHQ8BAf8EBAMCB4AwFgYDVR0lAQH/BAwwCgYIKwYBBQUHAwIwDQYJ\nKoZIhvcNAQEFBQADggEBAJ1kDQ/3sSt5ffmu4wwlfQWYGPT0a5kD4m84QDLabpa0\nPaiVsL/E6OKeC67i/oJWRz5A/TbsNCa2R5m1BUIgHQ2T/3Uormc3GI243885pnZn\nAP2Z0pr9CWQG1jPNBBVgRNt2IDteYQ2jH4ef8BbeKNQjbvr0fU/Uq98MukoeknrE\nWgM79uflCN1BAiiNijLtPJKzSve/yEU4HylDpwCdVOsETBKgGssDPU18Vv0/nT/3\nksFEXSdM+yBW7mHtjbLVlKy4BLcz2hxezBpttHwibFHGO56y8DLY2+dp/3wouVuj\nWOYJ/FbO2w8NR/dsRHH928nmhvObDwor6hhZxnYTbMQ=\n-----END CERTIFICATE-----\n",
 "otherKey3": "-----BEGIN CERTIFICATE-----\nMIIDHDCCAgSgAwIBAgIIVhDTved+WaMwDQYJKoZIhvcNAQEFBQAwMTEvMC0GA1UE\nAxMmc2VjdXJldG9rZW4uc3lzdGVtLmdzZXJ2aWNlYWNjb3VudC5jb20wHhcNMTcw\nNTI2MDA0NTI2WhcNMTcwNTI5MDExNTI2WjAxMS8wLQYDVQQDEyZzZWN1cmV0b2tl\nbi5zeXN0ZW0uZ3NlcnZpY2VhY2NvdW50LmNvbTCCASIwDQYJKoZIhvcNAQEBBQAD\nggEPADCCAQoCggEBAK4hcVb3XwyGDe0MtnxDqV8HZNYsG8cn6g23or7qDCKrK/UF\nw5Xj3+nxGuCTVJFol0hF2GpIvSTTkhiwVRDXnwnWJ7fupz1v8SEnyfGupbuwrgdD\nXRgVJZVGaUU53lXm8rbBQTXCtIKfG1mBELw465bpdcQRvL5uuV6bH6KSOMCZyA0k\nj4ROYPhQ2yrLXuN6kL8K+u6PK5T4veKeebLXfgcuKoGnngcvfiBeS3IbBbNYPeP3\ni0zxnAATd1fIo4THqcPtrCuhXaiObhNi5OKb6Ea6niukPkGIBmMQSK/2ytNy8sKl\n6ToDL+yD5miNgEWf5MLDfw7cJLYQ/HDTDObbE3UCAwEAAaM4MDYwDAYDVR0TAQH/\nBAIwADAOBgNVHQ8BAf8EBAMCB4AwFgYDVR0lAQH/BAwwCgYIKwYBBQUHAwIwDQYJ\nKoZIhvcNAQEFBQADggEBABJXw0qWcLbuHx5Xvs/CWQZbP9Eh72a8wKEK8DQP9ZxL\n8HTCpmTAYLx8mawiIOMngyrTm0ruPJCDTudsYCH9JnF33IMdfBpGYVC+J/T5+xlY\nKr+hAsalGGRHAjk+qQQ7VFxBGmlax9245l5OQSUy8rFvgD3N9sms4qkrwLP/90k8\nv7MPEYZH+/FvuXpCkfDIoUXX0I1FI6n4cWB24iYmeOqbBa85J7OfLuowni6HxvQQ\nh0KYkUqDNr/eEKoarbZZSqSrOq+uFtvCPxOixNTIcnykXTYmqBUD5vTTXKMhE/LB\n0b2EPKxFgZNRbUi5Ambo9EN6cUMLQ8kGSvrFSbwzg1U=\n-----END CERTIFICATE-----\n"
}`))
	}
)

func init() {
	var i big.Int
	i.SetString("269704609338850787826020655232726624618669490053633332458349776"+
		"0495124248924896100864530136128385258866309743300573262347231088573304816"+
		"5204610275235972605938766966332753162148808479299980044079410832664045762"+
		"6073132600734125029326300514972268883690287089048093860688063562026130780"+
		"5346220556482283692786045506782915268037302230139718847257970567513732177"+
		"1065742220716820417452252143656512820818136570898489694926617942928350867"+
		"6573840359620364892146446436897140231572372628385589853119286851169179430"+
		"9081625222541270629188074557316963892025466680832708302231208304464468201"+
		"1115262863896350105900343024764690353373761", 10)
	fakeKey = &rsa.PublicKey{
		N: &i,
		E: 65537,
	}
}

func TestKeyCache(t *testing.T) {
	hc, close := newTestServer(fakeKeyHandler)
	defer close()

	tests := []struct {
		desc string
		kc   *keyCache
		kid  string
		key  *rsa.PublicKey
		err  error
	}{
		{
			desc: "cache hit",
			kc: &keyCache{
				exp:  timeNow().Add(time.Minute),
				keys: map[string]*rsa.PublicKey{kidFound: fakeKey},
			}, kid: kidFound,
			key: fakeKey,
		},
		{
			desc: "cache valid, miss",
			kc: &keyCache{
				exp:  timeNow().Add(time.Minute),
				keys: map[string]*rsa.PublicKey{kidFound: fakeKey},
			}, kid: kidNotFound,
			err: ErrKeyNotFound,
		},
		{
			desc: "cache expired, hit",
			kc: &keyCache{
				hc: hc,
			},
			kid: kidFound,
			key: fakeKey,
		},
		{
			desc: "cache expired, miss",
			kc: &keyCache{
				hc: hc,
			},
			kid: kidNotFound,
			err: ErrKeyNotFound,
		},
		{
			desc: "missing http client",
			kc:   &keyCache{},
			kid:  kidFound,
			err:  fmt.Errorf("no http client defined on key cache"),
		},
	}

	for _, tt := range tests {
		key, err := tt.kc.get(tt.kid)

		if tt.err == nil {
			if err != nil {
				t.Errorf("TestKeyCache unexpected error, got: %v, want: %v", err, tt.err)
			}
		} else {
			if err == nil || err.Error() != tt.err.Error() {
				t.Errorf("TestKeyCache unexpected error, got: %v, want: %v", err, tt.err)
			}
		}

		if tt.key == nil {
			if key != nil {
				t.Errorf("TestKeyCache unexpected key returned, got: %v, want: %v", key, tt.key)
			}
		} else {
			if key == nil || key.N.Cmp(tt.key.N) != 0 || key.E != tt.key.E {
				t.Errorf("TestKeyCache unexpected key returned, got: %v, want: %v", key, tt.key)
			}
		}
	}
}

func newTestServer(handler func(w http.ResponseWriter, r *http.Request)) (*http.Client, func()) {
	ts := httptest.NewTLSServer(http.HandlerFunc(handler))
	tlsConf := &tls.Config{InsecureSkipVerify: true}
	tr := &http.Transport{
		TLSClientConfig: tlsConf,
		DialTLS: func(netw, addr string) (net.Conn, error) {
			return tls.Dial("tcp", ts.Listener.Addr().String(), tlsConf)
		},
	}
	return &http.Client{Transport: tr}, func() {
		tr.CloseIdleConnections()
		ts.Close()
	}
}
