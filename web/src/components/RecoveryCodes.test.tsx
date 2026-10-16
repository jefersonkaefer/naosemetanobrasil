import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { RecoveryCodes } from "./RecoveryCodes";

test("continuar exige confirmação explícita de que os códigos foram guardados", async () => {
  const user = userEvent.setup();
  const next = vi.fn();
  render(<RecoveryCodes codes={["NSR-example-code"]} onContinue={next} />);
  const button = screen.getByRole("button", {
    name: "Continuar para meu perfil",
  });
  expect(button).toBeDisabled();
  await user.click(button);
  expect(next).not.toHaveBeenCalled();
  await user.click(
    screen.getByRole("checkbox", {
      name: "Guardei meus códigos em um lugar seguro.",
    }),
  );
  await user.click(button);
  expect(next).toHaveBeenCalledOnce();
  expect(sessionStorage.length).toBe(0);
  expect(localStorage.length).toBe(0);
});

test("confirmar os códigos não libera a navegação antes de terminar o acesso", async () => {
  const user = userEvent.setup();
  const next = vi.fn();
  const view = render(
    <RecoveryCodes codes={["NSR-example-code"]} onContinue={next} busy />,
  );
  await user.click(screen.getByRole("checkbox"));
  expect(
    screen.getByRole("button", { name: "Continuar para meu perfil" }),
  ).toBeDisabled();
  view.rerender(
    <RecoveryCodes codes={["NSR-example-code"]} onContinue={next} />,
  );
  await user.click(
    screen.getByRole("button", { name: "Continuar para meu perfil" }),
  );
  expect(next).toHaveBeenCalledOnce();
});
