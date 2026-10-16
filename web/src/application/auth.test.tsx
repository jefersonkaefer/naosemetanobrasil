import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { AuthProvider, useAuth } from "./auth";
import {
  PlatformError,
  type PlatformGateway,
  type SessionView,
} from "../domain/platform";

test("restauração antiga não apaga um login concluído", async () => {
  let reject!: (error: Error) => void;
  const initial = new Promise<SessionView>((_, fail) => {
    reject = fail;
  });
  const session: SessionView = {
    user: {
      id: "user",
      username: "pessoa",
      display_name: "",
      role: "member",
      created_at: "2026-10-16",
    },
    instagram_profile: null,
  };
  const gateway = {
    current: vi.fn().mockReturnValueOnce(initial).mockResolvedValue(session),
    login: vi.fn().mockResolvedValue(undefined),
  } as unknown as PlatformGateway;
  function Probe() {
    const auth = useAuth();
    return (
      <>
        <p>{auth.session?.user.username || "anônimo"}</p>
        <button
          onClick={() =>
            void auth.login({ username: "pessoa", password: "password" })
          }
        >
          Entrar
        </button>
      </>
    );
  }
  render(
    <AuthProvider gateway={gateway}>
      <Probe />
    </AuthProvider>,
  );
  await userEvent.setup().click(screen.getByRole("button", { name: "Entrar" }));
  await screen.findByText("pessoa");
  await act(async () => {
    reject(new PlatformError("unauthenticated", "Não autenticado", 401));
  });
  await waitFor(() => expect(screen.getByText("pessoa")).toBeInTheDocument());
});

test("resposta de conta iniciada durante logout não restaura dados privados", async () => {
  let finishLogout!: () => void;
  let finishRefresh!: (value: SessionView) => void;
  const session: SessionView = {
    user: {
      id: "user",
      username: "pessoa",
      display_name: "",
      role: "member",
      created_at: "2026-10-16",
    },
    instagram_profile: null,
  };
  const gateway = {
    current: vi
      .fn()
      .mockResolvedValueOnce(session)
      .mockReturnValue(
        new Promise<SessionView>((resolve) => {
          finishRefresh = resolve;
        }),
      ),
    logout: vi.fn().mockReturnValue(
      new Promise<void>((resolve) => {
        finishLogout = resolve;
      }),
    ),
  } as unknown as PlatformGateway;
  function Probe() {
    const auth = useAuth();
    return (
      <>
        <p>{auth.session?.user.username || "anônimo"}</p>
        <button onClick={() => void auth.logout()}>Sair</button>
        <button onClick={() => void auth.refresh()}>Atualizar</button>
      </>
    );
  }
  sessionStorage.setItem("nsm:report-draft:user:user", "texto privado");
  render(
    <AuthProvider gateway={gateway}>
      <Probe />
    </AuthProvider>,
  );
  await screen.findByText("pessoa");
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Sair" }));
  await user.click(screen.getByRole("button", { name: "Atualizar" }));
  await act(async () => {
    finishLogout();
  });
  await screen.findByText("anônimo");
  await act(async () => {
    finishRefresh(session);
  });
  expect(screen.getByText("anônimo")).toBeInTheDocument();
  expect(sessionStorage.getItem("nsm:report-draft:user:user")).toBeNull();
});
