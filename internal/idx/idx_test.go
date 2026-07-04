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
 {"Date":"2026-06-29T00:00:00","StockCode":"BBCA","StockName":"Bank Central Asia Tbk.","OpenPrice":6175,"High":6200,"Low":5925,"Close":5925,"Change":-250,"Volume":189886600,"ListedShares":122042299500,"ForeignBuy":93315300,"ForeignSell":163734800}
]}`

const profileJSON = `{"Profiles":[
 {"KodeEmiten":"BBCA","NamaEmiten":"PT Bank Central Asia Tbk.","Alamat":"Menara BCA","Sektor":"Financials","SubSektor":"Banks","Industri":"Banks","SubIndustri":"Banks","PapanPencatatan":"Main","TanggalPencatatan":"2000-05-31T00:00:00","Website":"www.bca.co.id","Email":"ir@bca.co.id","Telepon":"021-23588000","KegiatanUsahaUtama":"Banking"}
],"PemegangSaham":[
 {"Nama":"PT Dwimuria Investama Andalan","Kategori":"More than 5%","Jumlah":67729950000,"Persentase":54.942,"Pengendali":true},
 {"Nama":"Masyarakat Non Warkat","Kategori":"Masyarakat Non Warkat","Jumlah":51940899478,"Persentase":42.134,"Pengendali":false},
 {"Nama":"Saham Treasury","Kategori":"Treasury Stock","Jumlah":432416000,"Persentase":0.351,"Pengendali":false}
],"Direktur":[
 {"Nama":"Gregory Hendra Lembong","Jabatan":"PRESIDENT DIRECTOR","Afiliasi":false}
],"Komisaris":[
 {"Nama":"Sumantri Slamet","Jabatan":"COMMISSIONER","Independen":true},
 {"Nama":"Jahja Setiaatmadja","Jabatan":"PRESIDENT COMMISIONER","Independen":false}
],"AnakPerusahaan":[
 {"Nama":"PT Asuransi Umum BCA","BidangUsaha":"Asuransi umum atau\r\nkerugian","Lokasi":"Jakarta","Persentase":100,"JumlahAset":3454384,"MataUang":"IDR","Satuan":"JUTAAN","StatusOperasi":"Beroperasi","TahunKomersil":"1989"},
 {"Nama":"PT Asuransi Jiwa BCA","BidangUsaha":"Asuransi Jiwa","Lokasi":"Jakarta","Persentase":90,"JumlahAset":4676146,"MataUang":"IDR","Satuan":"JUTAAN","StatusOperasi":"Beroperasi","TahunKomersil":"2014"}
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
	if d.ListedShares != 122042299500 || d.StockName != "Bank Central Asia Tbk." {
		t.Errorf("ListedShares/StockName = %v/%q", d.ListedShares, d.StockName)
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

func TestShareholders(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetCompanyProfilesDetail": profileJSON}}
	c := New(f, nil)

	sh, err := c.Shareholders(context.Background(), "BBCA")
	if err != nil {
		t.Fatalf("Shareholders: %v", err)
	}
	if len(sh) != 3 {
		t.Fatalf("got %d shareholders, want 3", len(sh))
	}
	// Sorted by percentage desc: Dwimuria (54.9) first.
	if sh[0].Name != "PT Dwimuria Investama Andalan" || !sh[0].Controller {
		t.Errorf("top holder = %+v, want Dwimuria as controller", sh[0])
	}
	if sh[0].Percentage < sh[1].Percentage || sh[1].Percentage < sh[2].Percentage {
		t.Errorf("not sorted desc by percentage: %v", []float64{sh[0].Percentage, sh[1].Percentage, sh[2].Percentage})
	}
	if sh[2].Category != "Treasury Stock" {
		t.Errorf("last = %+v, want Treasury Stock", sh[2])
	}
}

func TestSubsidiaries(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetCompanyProfilesDetail": profileJSON}}
	c := New(f, nil)

	subs, err := c.Subsidiaries(context.Background(), "BBCA")
	if err != nil {
		t.Fatalf("Subsidiaries: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("got %d subsidiaries, want 2", len(subs))
	}
	// Sorted by ownership desc: 100% before 90%.
	if subs[0].OwnershipPct != 100 || subs[1].OwnershipPct != 90 {
		t.Errorf("not sorted by ownership: %v", []float64{subs[0].OwnershipPct, subs[1].OwnershipPct})
	}
	// Embedded CRLF in line-of-business must be collapsed.
	if subs[0].LineOfBusiness != "Asuransi umum atau kerugian" {
		t.Errorf("LineOfBusiness = %q, want CRLF collapsed", subs[0].LineOfBusiness)
	}
	if subs[0].AssetUnit != "JUTAAN" || subs[0].TotalAssets != 3454384 {
		t.Errorf("asset fields = %+v", subs[0])
	}
}

func TestManagement(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetCompanyProfilesDetail": profileJSON}}
	c := New(f, nil)

	m, err := c.Management(context.Background(), "BBCA")
	if err != nil {
		t.Fatalf("Management: %v", err)
	}
	if len(m.Directors) != 1 || len(m.Commissioners) != 2 {
		t.Fatalf("got %d directors / %d commissioners, want 1 / 2", len(m.Directors), len(m.Commissioners))
	}
	if m.Directors[0].Position != "PRESIDENT DIRECTOR" {
		t.Errorf("director = %+v", m.Directors[0])
	}
	// Sumantri is the independent commissioner in the fixture.
	if !m.Commissioners[0].Independent || m.Commissioners[1].Independent {
		t.Errorf("independence flags wrong: %+v", m.Commissioners)
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
