package plugin

import (
	"av-capture/model"
	"av-capture/number"
	"av-capture/searcher/decoder"
	"av-capture/searcher/utils"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

const (
	defaultAvsoxSearchExpr = `//*[@id="waterfall"]/div/a/@href`
)

type avsox struct {
	DefaultPlugin
}

func (p *avsox) OnMakeHTTPRequest(ctx *PluginContext, number *number.Number) (*http.Request, error) {
	ctx.SetKey("number_info", number)
	return http.NewRequest(http.MethodGet, "https://avsox.click", nil) //返回一个假的request
}

func (p *avsox) OnHandleHTTPRequest(ctx *PluginContext, invoker HTTPInvoker, _ *http.Request) (*http.Response, error) {
	number := ctx.GetKeyOrDefault("number_info", nil).(*number.Number)
	num := strings.ToUpper(number.Number())
	if strings.Contains(num, "FC2") && !strings.Contains(num, "FC2-PPV") {
		num = strings.ReplaceAll(num, "FC2", "FC2-PPV")
	}
	tryList := p.generateTryList(num)
	logger := logutil.GetLogger(ctx.GetContext()).With(zap.String("plugin", "avsox"))
	logger.Debug("build try list succ", zap.Int("count", len(tryList)), zap.Strings("list", tryList))
	var link string
	var ok bool
	var err error
	for _, item := range tryList {
		link, ok, err = p.trySearchByNumber(ctx, invoker, item)
		if err != nil {
			logger.Error("try search number failed", zap.Error(err), zap.String("number", item))
			continue
		}
		if !ok {
			logger.Debug("search item not found, try next", zap.String("number", item))
			continue
		}
		break
	}
	if len(link) == 0 {
		return nil, fmt.Errorf("unable to find match number")
	}
	uri := "https:" + link
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("make request failed, err:%w", err)
	}
	return invoker(ctx, req)
}

func (p *avsox) generateTryList(num string) []string {
	tryList := make([]string, 0, 5)
	tryList = append(tryList, num)
	if strings.Contains(tryList[len(tryList)-1], "-") {
		tryList = append(tryList, strings.ReplaceAll(tryList[len(tryList)-1], "-", "_"))
	}
	if strings.Contains(tryList[len(tryList)-1], "_") {
		tryList = append(tryList, strings.ReplaceAll(tryList[len(tryList)-1], "_", ""))
	}
	return tryList
}

func (p *avsox) trySearchByNumber(ctx *PluginContext, invoker HTTPInvoker, number string) (string, bool, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://avsox.click/cn/search/%s", number), nil)
	if err != nil {
		return "", false, err
	}
	rsp, err := invoker(ctx, req)
	if err != nil {
		return "", false, err
	}
	defer rsp.Body.Close()
	tree, err := utils.ReadDataAsHTMLTree(rsp)
	if err != nil {
		return "", false, err
	}
	tmp := decoder.DecodeList(tree, defaultAvsoxSearchExpr)
	res := make([]string, 0, len(tmp))
	for _, item := range tmp {
		if strings.Contains(item, "movie") {
			res = append(res, item)
		}
	}
	if len(res) == 0 {
		return "", false, fmt.Errorf("no search item found")
	}
	if len(res) > 5 { //5个以内, 认为其还是ok的, 避免部分番号就是有重复的
		return "", false, fmt.Errorf("too much search item, cnt:%d", len(res))
	}
	return res[0], true, nil
}

func (p *avsox) OnDecodeHTTPData(ctx *PluginContext, data []byte) (*model.AvMeta, bool, error) {
	os.WriteFile("test_data", data, 0644)
	dec := decoder.XPathHtmlDecoder{
		NumberExpr:          `//span[contains(text(),"识别码:")]/../span[2]/text()`,
		TitleExpr:           `/html/body/div[2]/h3/text()`,
		PlotExpr:            "",
		ActorListExpr:       `//a[@class="avatar-box"]/span/text()`,
		ReleaseDateExpr:     `//span[contains(text(),"发行时间:")]/../text()`,
		DurationExpr:        `//p[span[contains(text(), "长度")]]/text()`,
		StudioExpr:          `//p[contains(text(),"制作商: ")]/following-sibling::p[1]/a/text()`,
		LabelExpr:           ``,
		DirectorExpr:        "",
		SeriesExpr:          `//p[contains(text(),"系列:")]/following-sibling::p[1]/a/text()`,
		GenreListExpr:       `//p[span[@class="genre"]]/span/a[contains(@href, "genre")]`,
		CoverExpr:           `/html/body/div[2]/div[1]/div[1]/a/img/@src`,
		PosterExpr:          "",
		SampleImageListExpr: "",
	}
	meta, err := dec.DecodeHTML(data,
		decoder.WithReleaseDateParser(p.doParseReleaseDate),
		decoder.WithDurationParser(p.doParseDuration),
		decoder.WithDefaultStringProcessor(strings.TrimSpace),
	)
	if err != nil {
		return nil, false, err
	}
	if len(meta.Number) == 0 {
		return nil, false, nil
	}
	return meta, true, nil
}

func (p *avsox) doParseDuration(v string) int64 {
	rs, _ := utils.ToDuration(v)
	return rs
}

func (p *avsox) doParseReleaseDate(v string) int64 {
	rs, _ := utils.ToTimestamp(v)
	return rs
}

func init() {
	Register(SSAvsox, PluginToCreator(&avsox{}))
}
