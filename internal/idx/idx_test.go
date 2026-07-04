package idx

import (
	"context"
	"strings"
	"testing"
)

// fakeFetcher returns canned bodies keyed by a substring of the requested URL.
type fakeFetcher struct {
	responses map[string]string
	calls     int
}

func (f *fakeFetcher) Get(_ context.Context, url string) ([]byte, error) {
	f.calls++
	for frag, body := range f.responses {
		if strings.Contains(url, frag) {
			return []byte(body), nil
		}
	}
	return nil, errNotFound
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (*notFoundError) Error() string { return "no canned response" }

const tradingJSON = `{"KodeEmiten":"BBCA","replies":[
 {"Date":"2026-06-29T00:00:00","StockCode":"BBCA","OpenPrice":6175,"High":6200,"Low":5925,"Close":5925,"Change":-250,"Volume":189886600,"ForeignBuy":93315300,"ForeignSell":163734800}
]}`

const profileJSON = `{"Profiles":[
 {"KodeEmiten":"BBCA","NamaEmiten":"PT Bank Central Asia Tbk.","Alamat":"Menara BCA","Sektor":"Financials","SubSektor":"Banks","Industri":"Banks","SubIndustri":"Banks","PapanPencatatan":"Main","TanggalPencatatan":"2000-05-31T00:00:00","Website":"www.bca.co.id","Email":"ir@bca.co.id","Telepon":"021-23588000","KegiatanUsahaUtama":"Banking"}
],"Dividen":[
 {"Jenis":"dti","TahunBuku":"2026","CashDividenPerSaham":20,"CashDividenPerSahamMU":"IDR","CashDividenTotal":0,"TotalSahamBonus":0,"Rasio1":0,"Rasio2":0,"TanggalCum":"2026-06-15T00:00:00","TanggalExRegulerDanNegosiasi":"2026-06-17T00:00:00","TanggalDPS":"2026-06-18T16:00:00","TanggalPembayaran":"2026-06-26T00:00:00"}
]}`

func TestTradingInfo(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetTradingInfoSS": tradingJSON}}
	c := New(f, nil) // nil cache: exercise the no-cache path

	days, err := c.TradingInfo(context.Background(), "bbca", 5)
	if err != nil {
		t.Fatalf("TradingInfo: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}
	d := days[0]
	if d.Date != "2026-06-29" {
		t.Errorf("Date = %q, want 2026-06-29 (time trimmed)", d.Date)
	}
	if d.Open != 6175 || d.Close != 5925 {
		t.Errorf("OHLC mismatch: open=%v close=%v", d.Open, d.Close)
	}
	// Foreign net = buy - sell = 93,315,300 - 163,734,800 = -70,419,500
	if d.ForeignNet != -70419500 {
		t.Errorf("ForeignNet = %v, want -70419500", d.ForeignNet)
	}
}

func TestCompanyProfile(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetCompanyProfilesDetail": profileJSON}}
	c := New(f, nil)

	p, err := c.CompanyProfile(context.Background(), "BBCA")
	if err != nil {
		t.Fatalf("CompanyProfile: %v", err)
	}
	if p.Name != "PT Bank Central Asia Tbk." || p.Sector != "Financials" {
		t.Errorf("profile = %+v", p)
	}
	if p.ListingDate != "2000-05-31" {
		t.Errorf("ListingDate = %q, want 2000-05-31", p.ListingDate)
	}
	if len(p.Dividends) != 1 || p.Dividends[0].CashPerShare != 20 || p.Dividends[0].Currency != "IDR" {
		t.Errorf("dividends = %+v", p.Dividends)
	}
	if p.Dividends[0].ExDate != "2026-06-17" {
		t.Errorf("ex date = %q", p.Dividends[0].ExDate)
	}
}

func TestDividends(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetCompanyProfilesDetail": profileJSON}}
	c := New(f, nil)

	divs, err := c.Dividends(context.Background(), "BBCA")
	if err != nil {
		t.Fatalf("Dividends: %v", err)
	}
	if len(divs) != 1 {
		t.Fatalf("got %d dividends, want 1", len(divs))
	}
	d := divs[0]
	if d.BookYear != "2026" || d.Type != "dti" || d.CashPerShare != 20 {
		t.Errorf("dividend = %+v", d)
	}
	if d.CumDate != "2026-06-15" || d.RecordDate != "2026-06-18" || d.PayDate != "2026-06-26" {
		t.Errorf("dividend dates = cum %q record %q pay %q", d.CumDate, d.RecordDate, d.PayDate)
	}
	// Rasio1/Rasio2 are both 0 in the fixture, so Ratio stays empty.
	if d.Ratio != "" {
		t.Errorf("Ratio = %q, want empty for a pure cash dividend", d.Ratio)
	}
}

func TestNormalizePeriod(t *testing.T) {
	cases := map[string]string{
		"tw1": "TW1", "q1": "TW1", "": "TW1",
		"tw3": "TW3", "audit": "Audit", "annual": "Audit", "tw4": "Audit",
	}
	for in, want := range cases {
		if got := normalizePeriod(in); got != want {
			t.Errorf("normalizePeriod(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildStaticURL(t *testing.T) {
	// File_Path with spaces and the characteristic double slash.
	in := "/Portals/0/StaticData//Laporan Keuangan Tahun 2026/TW1/BBCA/instance.zip"
	got := buildStaticURL(in)
	want := "https://www.idx.co.id/Portals/0/StaticData//Laporan%20Keuangan%20Tahun%202026/TW1/BBCA/instance.zip"
	if got != want {
		t.Errorf("buildStaticURL:\n got %q\nwant %q", got, want)
	}
}
