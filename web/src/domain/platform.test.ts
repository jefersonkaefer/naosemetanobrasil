import { expect, test } from "vitest";
import { activeStatus, type Challenge } from "./platform";

test("prazo vencido nunca aparece como disponível para confirmação", () => {
  const challenge: Challenge = {
    id: "id",
    expected_handle: "pessoa",
    observed_handle: "pessoa",
    status: "awaiting_confirmation",
    created_at: "2026-10-16T12:00:00Z",
    expires_at: "2026-10-17T12:00:00Z",
    sent_at: null,
  };
  expect(activeStatus(challenge, Date.parse(challenge.expires_at))).toBe(
    "expired",
  );
  expect(
    activeStatus(
      { ...challenge, status: "verified" },
      Date.parse(challenge.expires_at),
    ),
  ).toBe("verified");
});
