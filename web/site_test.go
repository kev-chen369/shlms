package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedSiteAssets(t *testing.T) {
	for _, tc := range []struct{ path, content string }{
		{"/", "万宝单生活"}, {"/styles.css", ".bottom-nav"}, {"/app.js", "loadHomeCoupons"}, {"/share.mjs", "copyPromotionShare"}, {"/promoter.mjs", "loadPromoterState"}, {"/promotion-flow.mjs", "createPromotionFlow"}, {"/promotion-flow-view.mjs", "mountPromotionFlow"},
	} {
		w := httptest.NewRecorder()
		Handler().ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), tc.content) {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
	}
}
