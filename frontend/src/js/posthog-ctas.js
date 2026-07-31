(function () {
  function captureCta(element) {
    if (!element || !window.Rowset?.hasAnalyticsConsent?.()) return;
    let destination = "";
    try {
      destination = new window.URL(element.href || element.action, window.location.origin).pathname;
    } catch (_error) {
      // An invalid destination is represented by the empty allowlisted value.
    }
    const properties = {
      event_version: 1,
      environment: window.Rowset.posthogEnvironment || "unknown",
      cta_name: element.dataset.posthogCta,
      cta_location: element.dataset.posthogCtaLocation || "unknown",
      destination,
      traffic_category: window.Rowset.posthogPageviewContext?.trafficCategory || "",
    };
    if (element.dataset.posthogCta === "signup") {
      properties.activation_stage = "landing_primary_cta";
      properties.activation_stage_order = 1;
    }
    window.posthog?.capture?.("rowset_marketing_cta_clicked", properties);
  }

  document.addEventListener("click", (event) => {
    const link = event.target.closest?.("a[data-posthog-cta]");
    captureCta(link);
  });

  document.addEventListener("submit", (event) => {
    const form = event.target.closest?.("form[data-posthog-cta]");
    if (!form) return;
    const sessionInput = form.querySelector?.("[data-posthog-session-id]");
    if (sessionInput && window.Rowset?.hasAnalyticsConsent?.()) {
      sessionInput.value = window.Rowset.posthogSessionId || "";
    }
    captureCta(form);
  });
})();
