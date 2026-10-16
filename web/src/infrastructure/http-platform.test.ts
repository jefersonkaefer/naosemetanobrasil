import { expect, test, vi } from "vitest";
import { HttpPlatform } from "./http-platform";

const csrf = "a".repeat(64);
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });

test("cookie é enviado pela mesma origem e CSRF acompanha a mutação protegida", async () => {
  const transport = vi
    .fn<typeof fetch>()
    .mockResolvedValueOnce(json({ csrf_token: csrf }))
    .mockResolvedValueOnce(
      json({ challenge: { id: "challenge" }, code: "NSM-secret" }),
    );
  const gateway = new HttpPlatform(transport);
  await gateway.login({
    username: "pessoa",
    password: "a-long-enough-password",
  });
  await gateway.issueChallenge("pessoa");
  const options = transport.mock.calls[1][1]!;
  expect(options.credentials).toBe("same-origin");
  expect(options.headers).toMatchObject({ "X-CSRF-Token": csrf });
  expect(sessionStorage.length).toBe(0);
  expect(localStorage.length).toBe(0);
});

test("resposta HTML de inicialização vira erro legível e não repete o envio", async () => {
  const transport = vi
    .fn<typeof fetch>()
    .mockResolvedValue(new Response("<html>Starting</html>", { status: 503 }));
  await expect(
    new HttpPlatform(transport).register({
      username: "pessoa",
      password: "long-password",
      display_name: "",
    }),
  ).rejects.toThrow("temporariamente indisponível");
  expect(transport).toHaveBeenCalledOnce();
});

test("falha de logout não apaga prematuramente o token CSRF da sessão", async () => {
  const transport = vi
    .fn<typeof fetch>()
    .mockResolvedValueOnce(json({ csrf_token: csrf }))
    .mockResolvedValueOnce(
      json({ error: { code: "unavailable", message: "Indisponível" } }, 503),
    )
    .mockResolvedValueOnce(json({ challenge: {} }));
  const gateway = new HttpPlatform(transport);
  await gateway.login({ username: "pessoa", password: "password" });
  await expect(gateway.logout()).rejects.toThrow("Indisponível");
  await gateway.issueChallenge("pessoa");
  expect(transport.mock.calls[2][1]!.headers).toMatchObject({
    "X-CSRF-Token": csrf,
  });
});

test("uma resposta 401 antiga não invalida uma sessão aberta depois", async () => {
  let resolve!: (response: Response) => void;
  const pending = new Promise<Response>((done) => {
    resolve = done;
  });
  const transport = vi
    .fn<typeof fetch>()
    .mockReturnValueOnce(pending)
    .mockResolvedValueOnce(json({ csrf_token: csrf }))
    .mockResolvedValueOnce(json({ challenge: {} }));
  const gateway = new HttpPlatform(transport);
  const old = gateway.current().catch(() => {});
  await gateway.login({ username: "pessoa", password: "password" });
  resolve(
    json(
      { error: { code: "unauthenticated", message: "Não autenticado" } },
      401,
    ),
  );
  await old;
  await gateway.issueChallenge("pessoa");
  expect(transport.mock.calls[2][1]!.headers).toMatchObject({
    "X-CSRF-Token": csrf,
  });
});

test("mutação sem sessão segura falha antes de acessar a rede", async () => {
  const transport = vi.fn<typeof fetch>();
  await expect(
    new HttpPlatform(transport).issueChallenge("pessoa"),
  ).rejects.toThrow("Entre novamente");
  expect(transport).not.toHaveBeenCalled();
});
