import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = fs.readFileSync(new URL("./openai-ads.js", import.meta.url), "utf8");

function loadOpenAiAds({
  consent = "",
  registrationCompleted = true,
  sharedConsentController = false,
  sessionStorage = new Map(),
} = {}) {
  const listeners = new Map();
  const calls = [];
  const banner = { hidden: true };
  const cookies = new Map(consent ? [["rowset_analytics_consent", consent]] : []);
  const buttons = new Map();
  const document = {
    readyState: "complete",
    querySelector(selector) {
      if (selector === "[data-analytics-consent]") return banner;
      if (!buttons.has(selector)) {
        buttons.set(selector, {
          addEventListener: (_name, callback) => listeners.set(selector, callback),
        });
      }
      return buttons.get(selector);
    },
  };
  Object.defineProperty(document, "cookie", {
    get: () => [...cookies].map(([name, value]) => `${name}=${value}`).join("; "),
    set: (value) => {
      const [pair] = value.split(";");
      const separator = pair.indexOf("=");
      cookies.set(pair.slice(0, separator), pair.slice(separator + 1));
    },
  });
  const Rowset = { openAiAds: { registrationCompleted } };
  if (sharedConsentController) {
    Rowset.hasAnalyticsConsent = () => consent === "granted";
  }
  const window = {
    Rowset,
    addEventListener: (name, callback) => listeners.set(name, callback),
    location: { protocol: "https:" },
    oaiq: (...args) => calls.push(args),
    sessionStorage: {
      getItem: (name) => sessionStorage.get(name) || null,
      removeItem: (name) => sessionStorage.delete(name),
      setItem: (name, value) => sessionStorage.set(name, value),
    },
  };

  vm.runInContext(source, vm.createContext({ document, window }));
  return {
    banner,
    calls,
    click: (selector) => listeners.get(selector)(),
    emit: (name) => listeners.get(name)(),
    getCookie: () => document.cookie,
    hasListener: (name) => listeners.has(name),
    sessionStorage,
  };
}

test("shows the shared consent choice when no preference exists", () => {
  const { banner } = loadOpenAiAds({ registrationCompleted: false });

  assert.equal(banner.hidden, false);
});

test("measures a completed registration when analytics consent is already granted", () => {
  const { calls } = loadOpenAiAds({ consent: "granted" });

  assert.deepEqual(JSON.parse(JSON.stringify(calls)), [
    ["measure", "registration_completed", { type: "customer_action" }],
  ]);
});

test("waits for consent before measuring a completed registration", () => {
  const integration = loadOpenAiAds();

  assert.deepEqual(integration.calls, []);
  integration.click("[data-analytics-consent-accept]");

  assert.deepEqual(JSON.parse(JSON.stringify(integration.calls)), [
    ["consent", true],
    ["measure", "registration_completed", { type: "customer_action" }],
  ]);
  assert.match(integration.getCookie(), /rowset_analytics_consent=granted/);
});

test("reuses the existing consent controller when PostHog owns the choice", () => {
  const integration = loadOpenAiAds({ sharedConsentController: true });

  assert.equal(integration.hasListener("[data-analytics-consent-accept]"), false);
  integration.emit("rowset:analytics-consent-granted");

  assert.deepEqual(JSON.parse(JSON.stringify(integration.calls)), [
    ["consent", true],
    ["measure", "registration_completed", { type: "customer_action" }],
  ]);
  assert.equal(integration.getCookie(), "");
});

test("never measures the same registration twice", () => {
  const integration = loadOpenAiAds({ consent: "granted" });

  integration.click("[data-analytics-consent-accept]");

  assert.equal(
    integration.calls.filter((call) => call[0] === "measure").length,
    1,
  );
});

test("declining consent keeps conversion measurement disabled", () => {
  const integration = loadOpenAiAds();

  integration.click("[data-analytics-consent-decline]");

  assert.deepEqual(integration.calls, [["consent", false]]);
  assert.match(integration.getCookie(), /rowset_analytics_consent=denied/);
});

test("keeps an unconsented registration pending across navigation", () => {
  const sessionStorage = new Map();
  loadOpenAiAds({ sessionStorage });

  const nextPage = loadOpenAiAds({
    registrationCompleted: false,
    sessionStorage,
  });
  nextPage.click("[data-analytics-consent-accept]");

  assert.deepEqual(JSON.parse(JSON.stringify(nextPage.calls)), [
    ["consent", true],
    ["measure", "registration_completed", { type: "customer_action" }],
  ]);
  assert.equal(sessionStorage.size, 0);
});

test("declining consent clears the pending registration", () => {
  const integration = loadOpenAiAds();

  integration.click("[data-analytics-consent-decline]");

  assert.equal(integration.sessionStorage.size, 0);
});
