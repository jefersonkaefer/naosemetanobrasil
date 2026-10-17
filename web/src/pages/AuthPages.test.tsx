import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";
import { expect, test, vi } from "vitest";
import { AuthProvider } from "../application/auth";
import {
  PlatformError,
  type PlatformGateway,
  type SessionView,
} from "../domain/platform";
import { LoginPage } from "./AuthPages";

function mountLogin(role: "member" | "admin", from?: string, restored = false) {
  const session: SessionView = {
    user: {
      id: "login-test-user",
      username: "pessoa",
      display_name: "Pessoa",
      role,
      created_at: "2026-10-17",
    },
    instagram_profile: null,
  };
  const current = vi.fn().mockResolvedValue(session);
  if (!restored)
    current.mockRejectedValueOnce(
      new PlatformError("unauthenticated", "Não autenticado", 401),
    );
  const gateway = {
    current,
    login: vi.fn().mockResolvedValue(undefined),
  } as unknown as PlatformGateway;
  render(
    <AuthProvider gateway={gateway}>
      <MemoryRouter initialEntries={[{ pathname: "/entrar", state: { from } }]}>
        <Routes>
          <Route path="/entrar" element={<LoginPage />} />
          <Route path="/conta" element={<h1>Minha conta de teste</h1>} />
          <Route path="/vincular" element={<h1>Vínculo de teste</h1>} />
          <Route
            path="/admin/verificacoes"
            element={<h1>Painel administrativo de teste</h1>}
          />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  );
  return gateway;
}

test.each([
  ["admin", undefined, "Painel administrativo de teste"],
  ["admin", "/vincular", "Painel administrativo de teste"],
  ["admin", "/admin/verificacoes", "Painel administrativo de teste"],
  ["member", undefined, "Minha conta de teste"],
  ["member", "/vincular", "Vínculo de teste"],
  ["member", "/admin/verificacoes", "Minha conta de teste"],
] as const)(
  "login de %s com origem %s abre %s",
  async (role, from, heading) => {
    mountLogin(role, from);
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Nome de usuário"), "pessoa");
    await user.type(
      screen.getByLabelText("Senha"),
      "Senha-sintetica-teste-123",
    );
    if (role === "admin" && from !== "/admin/verificacoes")
      await user.click(
        screen.getByRole("button", { name: "Acesso administrativo" }),
      );
    if (role === "admin" || from === "/admin/verificacoes")
      await user.type(
        screen.getByLabelText("Código do autenticador"),
        "123456",
      );
    await user.click(
      screen.getByRole("button", { name: "Entrar na minha conta" }),
    );
    await expect(
      screen.findByRole("heading", { name: heading }),
    ).resolves.toBeInTheDocument();
  },
);

test("sessão administrativa restaurada também abre o painel", async () => {
  mountLogin("admin", "/vincular", true);
  expect(
    await screen.findByRole("heading", {
      name: "Painel administrativo de teste",
    }),
  ).toBeInTheDocument();
});

test("entrada pela área administrativa já mostra o campo do autenticador", () => {
  mountLogin("admin", "/admin/verificacoes");
  expect(screen.getByLabelText("Código do autenticador")).toBeVisible();
});
