package idx

import (
	"context"
	"testing"
)

const brokerJSON = `{"recordsTotal":3,"data":[
 {"IDFirm":"AD","FirmName":"Sukadana Prima Sekuritas","Volume":431900,"Value":147267300,"Frequency":72},
 {"IDFirm":"YP","FirmName":"Mirae Asset Sekuritas","Volume":9000000,"Value":8500000000,"Frequency":50000},
 {"IDFirm":"CC","FirmName":"Mandiri Sekuritas","Volume":5000000,"Value":3200000000,"Frequency":21000}
]}`

func TestBrokerSummary(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetBrokerSummary": brokerJSON}}
	c := New(f, nil)

	b, err := c.BrokerSummary(context.Background())
	if err != nil {
		t.Fatalf("BrokerSummary: %v", err)
	}
	if len(b) != 3 {
		t.Fatalf("got %d brokers, want 3", len(b))
	}
	// Sorted by value desc: YP (8.5B) > CC (3.2B) > AD (0.147B).
	if b[0].Code != "YP" || b[1].Code != "CC" || b[2].Code != "AD" {
		t.Errorf("not sorted by value: %s %s %s", b[0].Code, b[1].Code, b[2].Code)
	}
	if b[0].Name != "Mirae Asset Sekuritas" || b[0].Value != 8500000000 {
		t.Errorf("top broker = %+v", b[0])
	}
}
