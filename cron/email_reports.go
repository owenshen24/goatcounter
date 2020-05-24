// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
)

func emailReports(ctx context.Context) error {
	db := zdb.MustGet(ctx)

	var ids []int64
	err := db.SelectContext(ctx, &ids,
		`select id from sites where settings->>'email_reports'::varchar != '0'`)
	if err != nil {
		return fmt.Errorf("cron.emailReports get sites: %w", err)
	}

	var sites goatcounter.Sites
	err = sites.ListIDs(ctx, ids...)
	if err != nil {
		return fmt.Errorf("cron.emailReports: %w", err)
	}

	// Note: maybe pool subsites in one email?
	for _, s := range sites {
		text, html, err := report(ctx, s)
		if err != nil {
			zlog.Field("site", s.ID).Errorf("cron.emailReports: %w", err)
			continue
		}

		zmail.Send("Report",
			mail.Address{Name: "GoatCounter report", Address: cfg.EmailFrom},
			zmail.To("TODO@TODO.TODO"),
			zmail.BodyText(text),
			zmail.BodyHTML(html))
	}

	return nil
}

type templateArgs struct {
	Site       goatcounter.Site
	PeriodName string
	Pages      goatcounter.HitStats
}

func report(ctx context.Context, s goatcounter.Site) ([]byte, []byte, error) {
	ctx = goatcounter.WithSite(ctx, &s)

	pn := map[int]string{
		-1: "first-time",
		1:  "weekly",
		2:  "biweekly",
		3:  "monthly",
	}[s.Settings.EmailReports.Int()]

	start := goatcounter.Now().Add(-7 * 24 * time.Hour)
	end := goatcounter.Now().Add(-14 * 24 * time.Hour)

	var pages goatcounter.HitStats
	_, _, _, _, _, _, err := pages.List(ctx, start, end, "", nil, true)
	if err != nil {
		return nil, nil, fmt.Errorf("cron.report: %w", err)
	}

	args := templateArgs{
		Site:       s,
		PeriodName: pn,
		Pages:      pages,
	}

	text, err := zhttp.ExecuteTpl("email_report.gotxt", args)
	if err != nil {
		return nil, nil, fmt.Errorf("cron.report text: %w", err)
	}
	html, err := zhttp.ExecuteTpl("email_report.gohtml", args)
	if err != nil {
		return nil, nil, fmt.Errorf("cron.report html: %w", err)
	}
	return text, html, nil
}
