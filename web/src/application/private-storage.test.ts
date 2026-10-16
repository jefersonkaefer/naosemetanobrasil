import { expect, test } from "vitest";
import {
  carryGuestDraft,
  clearPrivateStorage,
  draftKey,
} from "./private-storage";

test("rascunhos de contas diferentes ficam separados e o de visitante acompanha o login", () => {
  sessionStorage.setItem(draftKey(), "rascunho do visitante");
  sessionStorage.setItem(draftKey("outra-conta"), "relato privado anterior");
  carryGuestDraft("conta-atual");
  expect(sessionStorage.getItem(draftKey("conta-atual"))).toBe(
    "rascunho do visitante",
  );
  expect(sessionStorage.getItem(draftKey("outra-conta"))).toBe(
    "relato privado anterior",
  );
  expect(sessionStorage.getItem(draftKey())).toBeNull();
});

test("expiração limpa dados autenticados; logout também limpa o rascunho anônimo", () => {
  sessionStorage.setItem(draftKey("pessoa"), "relato privado");
  sessionStorage.setItem("nsm:challenge:pessoa", "id-privado");
  sessionStorage.setItem(
    "nsm:report-draft",
    "rascunho antigo sem dono identificado",
  );
  sessionStorage.setItem(draftKey(), "texto antes do cadastro");
  sessionStorage.setItem("preferencia-do-navegador", "manter");
  clearPrivateStorage();
  expect(sessionStorage.getItem(draftKey("pessoa"))).toBeNull();
  expect(sessionStorage.getItem("nsm:challenge:pessoa")).toBeNull();
  expect(sessionStorage.getItem("nsm:report-draft")).toBeNull();
  expect(sessionStorage.getItem(draftKey())).toBe("texto antes do cadastro");
  clearPrivateStorage(true);
  expect(sessionStorage.getItem(draftKey())).toBeNull();
  expect(sessionStorage.getItem("preferencia-do-navegador")).toBe("manter");
});
