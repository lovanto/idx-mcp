package idx

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// CompanyProfile is the cleaned subset of GetCompanyProfilesDetail most useful
// for analysis. The raw endpoint also returns directors, shareholders,
// dividends, subsidiaries, etc. — dividends are exposed via Dividends here (and
// the get_dividends tool), others omitted for now.
type CompanyProfile struct {
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	Address      string     `json:"address"`
	Sector       string     `json:"sector"`
	SubSector    string     `json:"sub_sector"`
	Industry     string     `json:"industry"`
	SubIndustry  string     `json:"sub_industry"`
	ListingBoard string     `json:"listing_board"`
	ListingDate  string     `json:"listing_date"`
	Website      string     `json:"website"`
	Email        string     `json:"email"`
	Phone        string     `json:"phone"`
	MainBusiness string     `json:"main_business"`
	Dividends    []Dividend `json:"dividends"`
}

// Shareholder is one ownership entry from the profile's PemegangSaham array.
// Categories include "More than 5%", "Treasury Stock", and public buckets
// (Masyarakat Warkat / Non Warkat). Controller marks the controlling holder.
type Shareholder struct {
	Name       string  `json:"name"`
	Category   string  `json:"category"`
	Shares     float64 `json:"shares"`
	Percentage float64 `json:"percentage"`
	Controller bool    `json:"controller"`
}

// Subsidiary is one entry from the profile's AnakPerusahaan array. TotalAssets
// is expressed in AssetUnit (e.g. "JUTAAN" = millions) of AssetCurrency.
type Subsidiary struct {
	Name            string  `json:"name"`
	LineOfBusiness  string  `json:"line_of_business"`
	Location        string  `json:"location"`
	OwnershipPct    float64 `json:"ownership_percent"`
	TotalAssets     float64 `json:"total_assets"`
	AssetCurrency   string  `json:"asset_currency"`
	AssetUnit       string  `json:"asset_unit"`
	OperationStatus string  `json:"operation_status"`
	CommercialYear  string  `json:"commercial_year"`
}

// BoardMember is a director or commissioner. Independent applies to
// commissioners, Affiliated to directors; each is false/omitted for the other.
type BoardMember struct {
	Name        string `json:"name"`
	Position    string `json:"position"`
	Independent bool   `json:"independent,omitempty"`
	Affiliated  bool   `json:"affiliated,omitempty"`
}

// Management is the board of directors and commissioners.
type Management struct {
	Directors     []BoardMember `json:"directors"`
	Commissioners []BoardMember `json:"commissioners"`
}

// Dividend is one corporate-action dividend entry. Source is the profile
// payload's Dividen array: the dedicated GetDividend endpoint returns 503, but
// the same data rides along in GetCompanyProfilesDetail. Note this array holds
// the most recently declared dividend(s), not full history.
type Dividend struct {
	BookYear     string  `json:"book_year"`              // fiscal year the dividend is for
	Type         string  `json:"type"`                   // IDX code, e.g. "dti" (interim cash)
	CashPerShare float64 `json:"cash_per_share"`         // cash dividend per share
	Currency     string  `json:"currency"`               // e.g. IDR
	CashTotal    float64 `json:"cash_total,omitempty"`   // total cash distributed
	BonusShares  float64 `json:"bonus_shares,omitempty"` // for stock/bonus dividends
	Ratio        string  `json:"ratio,omitempty"`        // "old:new" ratio for bonus shares
	CumDate      string  `json:"cum_date"`               // last day to buy with dividend rights
	ExDate       string  `json:"ex_date"`                // ex-dividend date (regular/nego market)
	RecordDate   string  `json:"record_date"`            // shareholders-of-record date
	PayDate      string  `json:"payment_date"`           // payment date
}

type rawProfileResponse struct {
	Profiles []struct {
		KodeEmiten         string `json:"KodeEmiten"`
		NamaEmiten         string `json:"NamaEmiten"`
		Alamat             string `json:"Alamat"`
		Sektor             string `json:"Sektor"`
		SubSektor          string `json:"SubSektor"`
		Industri           string `json:"Industri"`
		SubIndustri        string `json:"SubIndustri"`
		PapanPencatatan    string `json:"PapanPencatatan"`
		TanggalPencatatan  string `json:"TanggalPencatatan"`
		Website            string `json:"Website"`
		Email              string `json:"Email"`
		Telepon            string `json:"Telepon"`
		KegiatanUsahaUtama string `json:"KegiatanUsahaUtama"`
	} `json:"Profiles"`
	Dividen []struct {
		Jenis                        string  `json:"Jenis"`
		TahunBuku                    string  `json:"TahunBuku"`
		CashDividenPerSaham          float64 `json:"CashDividenPerSaham"`
		CashDividenPerSahamMU        string  `json:"CashDividenPerSahamMU"`
		CashDividenTotal             float64 `json:"CashDividenTotal"`
		TotalSahamBonus              float64 `json:"TotalSahamBonus"`
		Rasio1                       float64 `json:"Rasio1"`
		Rasio2                       float64 `json:"Rasio2"`
		TanggalCum                   string  `json:"TanggalCum"`
		TanggalExRegulerDanNegosiasi string  `json:"TanggalExRegulerDanNegosiasi"`
		TanggalDPS                   string  `json:"TanggalDPS"`
		TanggalPembayaran            string  `json:"TanggalPembayaran"`
	} `json:"Dividen"`
	PemegangSaham []struct {
		Nama       string  `json:"Nama"`
		Kategori   string  `json:"Kategori"`
		Jumlah     float64 `json:"Jumlah"`
		Persentase float64 `json:"Persentase"`
		Pengendali bool    `json:"Pengendali"`
	} `json:"PemegangSaham"`
	Direktur []struct {
		Nama     string `json:"Nama"`
		Jabatan  string `json:"Jabatan"`
		Afiliasi bool   `json:"Afiliasi"`
	} `json:"Direktur"`
	Komisaris []struct {
		Nama       string `json:"Nama"`
		Jabatan    string `json:"Jabatan"`
		Independen bool   `json:"Independen"`
	} `json:"Komisaris"`
	AnakPerusahaan []struct {
		Nama          string  `json:"Nama"`
		BidangUsaha   string  `json:"BidangUsaha"`
		Lokasi        string  `json:"Lokasi"`
		Persentase    float64 `json:"Persentase"`
		JumlahAset    float64 `json:"JumlahAset"`
		MataUang      string  `json:"MataUang"`
		Satuan        string  `json:"Satuan"`
		StatusOperasi string  `json:"StatusOperasi"`
		TahunKomersil string  `json:"TahunKomersil"`
	} `json:"AnakPerusahaan"`
}

// fetchProfileRaw retrieves and decodes GetCompanyProfilesDetail, shared by the
// profile and dividend methods so a single request/cache entry serves both.
func (c *Client) fetchProfileRaw(ctx context.Context, code string) (*rawProfileResponse, error) {
	code = normalizeCode(code)
	if code == "" {
		return nil, fmt.Errorf("empty emiten code")
	}
	u := fmt.Sprintf("%s/primary/ListedCompany/GetCompanyProfilesDetail?KodeEmiten=%s&language=en-us",
		baseURL, url.QueryEscape(code))
	key := "profile:" + code

	var raw rawProfileResponse
	if err := c.getJSON(ctx, key, u, ttlProfile, &raw); err != nil {
		return nil, err
	}
	return &raw, nil
}

// CompanyProfile returns the profile for an emiten code (e.g. "BBCA").
func (c *Client) CompanyProfile(ctx context.Context, code string) (*CompanyProfile, error) {
	raw, err := c.fetchProfileRaw(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(raw.Profiles) == 0 {
		return nil, fmt.Errorf("no profile found for %q", normalizeCode(code))
	}

	p := raw.Profiles[0]
	return &CompanyProfile{
		Code:         p.KodeEmiten,
		Name:         p.NamaEmiten,
		Address:      strings.TrimSpace(p.Alamat),
		Sector:       p.Sektor,
		SubSector:    p.SubSektor,
		Industry:     p.Industri,
		SubIndustry:  p.SubIndustri,
		ListingBoard: p.PapanPencatatan,
		ListingDate:  trimDate(p.TanggalPencatatan),
		Website:      p.Website,
		Email:        p.Email,
		Phone:        p.Telepon,
		MainBusiness: strings.TrimSpace(p.KegiatanUsahaUtama),
		Dividends:    dividendsFromRaw(raw),
	}, nil
}

// Dividends returns the declared dividend(s) for an emiten code. Backed by the
// profile payload, so it reflects the most recently declared dividend(s), not
// the full historical series.
func (c *Client) Dividends(ctx context.Context, code string) ([]Dividend, error) {
	raw, err := c.fetchProfileRaw(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(raw.Profiles) == 0 {
		return nil, fmt.Errorf("no profile found for %q", normalizeCode(code))
	}
	return dividendsFromRaw(raw), nil
}

// Shareholders returns the ownership breakdown for an emiten code, sorted by
// percentage held (largest first). Backed by the profile payload.
func (c *Client) Shareholders(ctx context.Context, code string) ([]Shareholder, error) {
	raw, err := c.fetchProfileRaw(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(raw.Profiles) == 0 {
		return nil, fmt.Errorf("no profile found for %q", normalizeCode(code))
	}

	out := make([]Shareholder, 0, len(raw.PemegangSaham))
	for _, s := range raw.PemegangSaham {
		out = append(out, Shareholder{
			Name:       strings.TrimSpace(s.Nama),
			Category:   s.Kategori,
			Shares:     s.Jumlah,
			Percentage: s.Persentase,
			Controller: s.Pengendali,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Percentage > out[j].Percentage })
	return out, nil
}

// Subsidiaries returns the consolidated subsidiaries for an emiten code, sorted
// by ownership percentage (largest first). Backed by the profile payload.
func (c *Client) Subsidiaries(ctx context.Context, code string) ([]Subsidiary, error) {
	raw, err := c.fetchProfileRaw(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(raw.Profiles) == 0 {
		return nil, fmt.Errorf("no profile found for %q", normalizeCode(code))
	}

	out := make([]Subsidiary, 0, len(raw.AnakPerusahaan))
	for _, s := range raw.AnakPerusahaan {
		out = append(out, Subsidiary{
			Name:            strings.TrimSpace(s.Nama),
			LineOfBusiness:  cleanText(s.BidangUsaha),
			Location:        strings.TrimSpace(s.Lokasi),
			OwnershipPct:    s.Persentase,
			TotalAssets:     s.JumlahAset,
			AssetCurrency:   s.MataUang,
			AssetUnit:       s.Satuan,
			OperationStatus: s.StatusOperasi,
			CommercialYear:  s.TahunKomersil,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].OwnershipPct > out[j].OwnershipPct })
	return out, nil
}

// Management returns the board of directors and commissioners for an emiten
// code. Backed by the profile payload.
func (c *Client) Management(ctx context.Context, code string) (*Management, error) {
	raw, err := c.fetchProfileRaw(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(raw.Profiles) == 0 {
		return nil, fmt.Errorf("no profile found for %q", normalizeCode(code))
	}

	m := &Management{
		Directors:     make([]BoardMember, 0, len(raw.Direktur)),
		Commissioners: make([]BoardMember, 0, len(raw.Komisaris)),
	}
	for _, d := range raw.Direktur {
		m.Directors = append(m.Directors, BoardMember{
			Name:       strings.TrimSpace(d.Nama),
			Position:   strings.TrimSpace(d.Jabatan),
			Affiliated: d.Afiliasi,
		})
	}
	for _, k := range raw.Komisaris {
		m.Commissioners = append(m.Commissioners, BoardMember{
			Name:        strings.TrimSpace(k.Nama),
			Position:    strings.TrimSpace(k.Jabatan),
			Independent: k.Independen,
		})
	}
	return m, nil
}

// cleanText trims whitespace and collapses embedded CR/LF (IDX free-text fields
// often contain "\r\n") into single spaces.
func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

// dividendsFromRaw maps the raw Dividen array into cleaned Dividend values.
func dividendsFromRaw(raw *rawProfileResponse) []Dividend {
	out := make([]Dividend, 0, len(raw.Dividen))
	for _, d := range raw.Dividen {
		div := Dividend{
			BookYear:     d.TahunBuku,
			Type:         d.Jenis,
			CashPerShare: d.CashDividenPerSaham,
			Currency:     d.CashDividenPerSahamMU,
			CashTotal:    d.CashDividenTotal,
			BonusShares:  d.TotalSahamBonus,
			CumDate:      trimDate(d.TanggalCum),
			ExDate:       trimDate(d.TanggalExRegulerDanNegosiasi),
			RecordDate:   trimDate(d.TanggalDPS),
			PayDate:      trimDate(d.TanggalPembayaran),
		}
		if d.Rasio1 != 0 || d.Rasio2 != 0 {
			div.Ratio = strconv.FormatFloat(d.Rasio1, 'f', -1, 64) + ":" +
				strconv.FormatFloat(d.Rasio2, 'f', -1, 64)
		}
		out = append(out, div)
	}
	return out
}
