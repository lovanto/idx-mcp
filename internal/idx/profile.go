package idx

import (
	"context"
	"fmt"
	"net/url"
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
