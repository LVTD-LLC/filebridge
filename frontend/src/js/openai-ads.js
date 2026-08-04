(function () {
  const Rowset = (window.Rowset = window.Rowset || {});
  const config = Rowset.openAiAds || {};
  const consentCookie = "rowset_analytics_consent";
  const pendingRegistrationKey = "rowset_openai_ads_registration_pending";
  const hasSharedConsentController = typeof Rowset.hasAnalyticsConsent === "function";
  let measurementAllowed = hasSharedConsentController
    ? Rowset.hasAnalyticsConsent()
    : cookieValue(consentCookie) === "granted";
  let registrationMeasured = false;

  function pendingRegistration() {
    try {
      if (config.registrationCompleted) {
        window.sessionStorage?.setItem(pendingRegistrationKey, "true");
      }
      return window.sessionStorage?.getItem(pendingRegistrationKey) === "true" ||
        Boolean(config.registrationCompleted);
    } catch (_error) {
      return Boolean(config.registrationCompleted);
    }
  }

  function clearPendingRegistration() {
    try {
      window.sessionStorage?.removeItem(pendingRegistrationKey);
    } catch (_error) {
      // Storage can be unavailable in privacy-restricted browser contexts.
    }
  }

  function cookieValue(name) {
    const prefix = `${name}=`;
    return (document.cookie || "")
      .split(";")
      .map((part) => part.trim())
      .find((part) => part.startsWith(prefix))
      ?.slice(prefix.length) || "";
  }

  function setCookie(name, value) {
    const secure = window.location.protocol === "https:" ? "; Secure" : "";
    document.cookie = `${name}=${value}; Path=/; Max-Age=${60 * 60 * 24 * 365}; SameSite=Lax${secure}`;
  }

  function showBanner(show) {
    const banner = document.querySelector("[data-analytics-consent]");
    if (banner) banner.hidden = !show;
  }

  function measureRegistration() {
    const hasPendingRegistration = pendingRegistration();
    if (!measurementAllowed || !hasPendingRegistration || registrationMeasured) return;
    try {
      window.oaiq?.("measure", "registration_completed", {
        type: "customer_action",
      });
      registrationMeasured = true;
      clearPendingRegistration();
    } catch (_error) {
      // Conversion measurement must never block or break signup.
    }
  }

  function grantConsent() {
    setCookie(consentCookie, "granted");
    if (!measurementAllowed) {
      measurementAllowed = true;
      window.oaiq?.("consent", true);
    }
    showBanner(false);
    measureRegistration();
  }

  function handleSharedConsentGranted() {
    measurementAllowed = true;
    window.oaiq?.("consent", true);
    measureRegistration();
  }

  function declineConsent() {
    setCookie(consentCookie, "denied");
    measurementAllowed = false;
    window.oaiq?.("consent", false);
    clearPendingRegistration();
    showBanner(false);
  }

  function initialize() {
    if (hasSharedConsentController) {
      window.addEventListener?.(
        "rowset:analytics-consent-granted",
        handleSharedConsentGranted,
      );
    } else {
      document
        .querySelector("[data-analytics-consent-accept]")
        ?.addEventListener("click", grantConsent);
      document
        .querySelector("[data-analytics-consent-decline]")
        ?.addEventListener("click", declineConsent);
      showBanner(!cookieValue(consentCookie));
    }
    measureRegistration();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initialize, { once: true });
  } else {
    initialize();
  }
})();
