import { execFileSync } from "node:child_process";
import { createHmac } from "node:crypto";
import { resolve } from "node:path";
import { expect, test } from "@playwright/test";

test("vínculo automático por webhook assinado, sem conferência administrativa", async ({
  page,
}) => {
  const sender = String(Date.now());
  const username = `auto_${sender}`;
  const password = "Senha-sintetica-para-webhook-123";
  const registration = await page.request.post("/v1/auth/register", {
    data: { username, password, display_name: "Teste automático" },
  });
  expect(registration.status()).toBe(201);
  expect(
    (
      await page.request.post("/v1/auth/login", {
        data: { username, password },
      })
    ).status(),
  ).toBe(200);
  await page.goto("/vincular");
  const handle = `perfil_e2e_${sender}`;
  await page.getByLabel("Qual é seu perfil no Instagram?").fill(handle);
  await page.getByRole("button", { name: "Gerar meu código" }).click();
  const code = await page.locator(".verification-code>code").innerText();
  const payload = JSON.stringify({
    object: "instagram",
    entry: [
      {
        id: "123456",
        messaging: [
          {
            sender: { id: sender },
            recipient: { id: "123456" },
            timestamp: Date.now(),
            message: { mid: `e2e-${sender}`, text: code },
          },
        ],
      },
    ],
  });
  const path = "/v1/webhooks/instagram";
  expect(
    (
      await page.request.post(path, {
        data: payload,
        headers: { "X-Hub-Signature-256": "sha256=invalid" },
      })
    ).status(),
  ).toBe(403);
  const headers = {
    "X-Hub-Signature-256": `sha256=${createHmac("sha256", process.env.META_APP_SECRET!).update(payload).digest("hex")}`,
  };
  expect(
    (await page.request.post(path, { data: payload, headers })).status(),
  ).toBe(200);
  expect(
    (await page.request.post(path, { data: payload, headers })).status(),
  ).toBe(200);
  await page.getByRole("button", { name: "Atualizar status" }).click();
  await expect(
    page.getByRole("heading", { name: "Este é seu perfil?" }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "Recebemos seu código e conferimos o remetente automaticamente.",
      { exact: false },
    ),
  ).toBeVisible();
  await page.getByRole("button", { name: `Confirmar @${handle}` }).click();
  await expect(
    page.getByRole("heading", { name: "Seu Instagram está conectado." }),
  ).toBeVisible();
  const me = await (await page.request.get("/v1/me")).json();
  expect(me.instagram_profile.verification_method).toBe("instagram_webhook");
  await page.screenshot({
    path: "test-results/instagram-automatic.png",
    fullPage: true,
  });
});

test("cadastro, vínculo por direct, conferência administrativa e recuperação com API real", async ({
  page,
  browser,
}) => {
  const username = `pessoa_${Date.now()}`;
  const password = "Senha-de-teste-com-12-caracteres";
  await page.goto("/cadastro");
  await page.getByLabel("Como você quer ser chamado?").fill("Ana");
  await page.getByLabel("Nome de usuário", { exact: true }).fill(username);
  await page.getByLabel("Senha", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Criar minha conta" }).click();
  await expect(
    page.getByRole("heading", { name: "Guarde seus códigos de recuperação" }),
  ).toBeVisible();
  const recovery = await page
    .getByTestId("recovery-code")
    .first()
    .textContent();
  await expect(
    page.getByRole("button", { name: "Continuar para meu perfil" }),
  ).toBeDisabled();
  await page
    .getByRole("checkbox", { name: "Guardei meus códigos em um lugar seguro." })
    .check();
  await page.getByRole("button", { name: "Continuar para meu perfil" }).click();
  await expect(page).toHaveURL(/\/vincular$/);
  const handle = `perfil_${Date.now()}`;
  await page.getByLabel("Qual é seu perfil no Instagram?").fill(handle);
  await page.getByRole("button", { name: "Gerar meu código" }).click();
  const code = await page.locator(".verification-code>code").textContent();
  expect(code).toMatch(/^NSM-/);
  expect(
    await page.evaluate(() =>
      JSON.stringify({
        session: { ...sessionStorage },
        local: { ...localStorage },
      }),
    ),
  ).not.toContain(code!);
  await page.getByRole("button", { name: "Já enviei o código" }).click();
  await expect(
    page.getByRole("heading", { name: "Mensagem enviada. Agora é aguardar." }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Mensagem enviada. Agora é aguardar." }),
  ).toBeVisible();
  await expect(page.locator(".verification-code")).toHaveCount(0);

  const adminContext = await browser.newContext({
    baseURL: process.env.E2E_BASE_URL,
  });
  try {
    const admin = await adminContext.newPage();
    await admin.goto("/entrar");
    await admin
      .getByLabel("Nome de usuário", { exact: true })
      .fill(process.env.E2E_ADMIN_USERNAME!);
    await admin
      .getByLabel("Senha", { exact: true })
      .fill(process.env.E2E_ADMIN_PASSWORD!);
    await admin.getByRole("button", { name: "Acesso administrativo" }).click();
    const otp = execFileSync(
      process.env.E2E_GO_BIN || "go",
      ["run", "-tags", "e2e", "./web/e2e/seed.go", "-code"],
      { cwd: resolve(process.cwd(), ".."), env: process.env },
    )
      .toString()
      .trim();
    await admin.getByLabel("Código do autenticador").fill(otp);
    await admin.getByRole("button", { name: "Entrar na minha conta" }).click();
    await admin.getByRole("link", { name: "Conferir vínculos" }).click();
    await admin.getByLabel("Código recebido no direct").fill(code!);
    await admin.getByLabel("Perfil real do remetente").fill(handle);
    await admin
      .getByLabel("Referência da conferência")
      .fill("Conferência simulada pelo cenário E2E local");
    await admin
      .getByRole("checkbox", {
        name: "Conferi a mensagem e o remetente na caixa oficial do Instagram.",
      })
      .check();
    await admin.getByRole("button", { name: "Registrar conferência" }).click();
    await expect(admin.getByRole("status")).toContainText(
      "A confirmação final cabe à pessoa.",
    );
  } finally {
    await adminContext.close();
  }

  await page.getByRole("button", { name: "Atualizar status" }).click();
  await expect(
    page.getByRole("heading", { name: "Este é seu perfil?" }),
  ).toBeVisible();
  await page.getByRole("button", { name: `Confirmar @${handle}` }).click();
  await expect(
    page.getByRole("heading", { name: "Seu Instagram está conectado." }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Seu Instagram está conectado." }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Sair da conta" }).click();
  await page.goto("/recuperar");
  await page.getByLabel("Nome de usuário", { exact: true }).fill(username);
  await page
    .getByLabel("Código de recuperação", { exact: true })
    .fill(recovery!);
  await page
    .getByLabel("Nova senha", { exact: true })
    .fill("Uma-nova-senha-para-o-teste");
  await page
    .getByLabel("Repita a nova senha")
    .fill("Uma-nova-senha-para-o-teste");
  await page.getByRole("button", { name: "Atualizar minha senha" }).click();
  await page.getByRole("link", { name: "Entrar com a nova senha" }).click();
  await page.getByLabel("Nome de usuário", { exact: true }).fill(username);
  await page
    .getByLabel("Senha", { exact: true })
    .fill("Uma-nova-senha-para-o-teste");
  await page.getByRole("button", { name: "Entrar na minha conta" }).click();
  await expect(page.getByRole("heading", { name: "Olá, Ana." })).toBeVisible();
  await expect(page.getByText(`@${handle}`, { exact: true })).toBeVisible();
});

test("mural identifica exemplos, filtra e funciona em tela de celular", async ({
  page,
}) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: /Não se Meta no Brasil/ }),
  ).toBeVisible();
  await page.screenshot({
    path: "test-results/home-desktop.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await expect(
    page.getByText("PRÉVIA DO MURAL", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
  await page.getByLabel("Buscar nos exemplos").fill("nenhuma-ocorrencia");
  await expect(
    page.getByRole("heading", { name: "Nenhum exemplo encontrado." }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Limpar filtros" }).click();
  await expect(page.locator(".report-card")).toHaveCount(3);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: "test-results/home-mobile.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Abrir menu" }).click();
  await page
    .getByRole("navigation", { name: "Navegação móvel" })
    .getByRole("link", { name: "Como funciona" })
    .click();
  await expect(
    page.getByRole("heading", { name: /Dar contexto/ }),
  ).toBeVisible();
});

test("acesso e recuperação são utilizáveis em celular e erros de login ficam visíveis", async ({
  page,
}) => {
  await page.setViewportSize({ width: 360, height: 780 });
  await page.goto("/conta");
  await expect(page).toHaveURL(/\/entrar$/);
  await expect(
    page.getByLabel("Nome de usuário", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(360);
  await page.screenshot({
    path: "test-results/login-mobile.png",
    fullPage: true,
  });
  await page
    .getByLabel("Nome de usuário", { exact: true })
    .fill("usuario_inexistente");
  await page
    .getByLabel("Senha", { exact: true })
    .fill("uma-senha-incorreta-mas-longa");
  await page.getByRole("button", { name: "Entrar na minha conta" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "Não foi possível autenticar.",
  );
  await page.getByRole("link", { name: "Esqueci minha senha" }).click();
  await expect(
    page.getByLabel("Código de recuperação", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(360);
});

test("rascunho permanece local e sobrevive à navegação para cadastro", async ({
  page,
}) => {
  await page.goto("/registrar");
  const description =
    "Uma experiência de teste que ainda não deve ser publicada.";
  await page.getByLabel("Conte em poucas palavras").fill(description);
  await page.getByRole("button", { name: "Salvar rascunho" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "Rascunho salvo" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Preparar meu acesso" }).click();
  await page.goBack();
  await expect(page.getByLabel("Conte em poucas palavras")).toHaveValue(
    description,
  );
});

test("rascunho acompanha cadastro e não aparece para outra conta após logout", async ({
  page,
}) => {
  const suffix = String(Date.now());
  const password = "Senha-sintetica-para-privacidade-123";
  const first = `priv_a_${suffix}`;
  const second = `priv_b_${suffix}`;
  const text = "Rascunho privado da primeira pessoa.";
  await page.goto("/registrar");
  await page.getByLabel("Conte em poucas palavras").fill(text);
  await page.getByRole("button", { name: "Salvar rascunho" }).click();
  expect(
    (
      await page.request.post("/v1/auth/register", {
        data: { username: first, password },
      })
    ).status(),
  ).toBe(201);
  await page.goto("/entrar");
  await page.getByLabel("Nome de usuário", { exact: true }).fill(first);
  await page.getByLabel("Senha", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Entrar na minha conta" }).click();
  await expect(page).toHaveURL(/\/conta$/);
  await page.goto("/registrar");
  await expect(page.getByLabel("Conte em poucas palavras")).toHaveValue(text);
  await page.getByRole("button", { name: "Sair da conta" }).click();
  await expect(page).toHaveURL(new URL("/", process.env.E2E_BASE_URL).href);
  expect(
    await page.evaluate(() =>
      Object.values({ ...sessionStorage }).some((value) =>
        value.includes("Rascunho privado"),
      ),
    ),
  ).toBe(false);
  expect(
    (
      await page.request.post("/v1/auth/register", {
        data: { username: second, password },
      })
    ).status(),
  ).toBe(201);
  expect(
    (
      await page.request.post("/v1/auth/login", {
        data: { username: second, password },
      })
    ).status(),
  ).toBe(200);
  await page.goto("/registrar");
  await expect(page.getByLabel("Conte em poucas palavras")).toHaveValue("");
});
