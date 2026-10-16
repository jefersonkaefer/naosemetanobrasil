import { useState } from "react";
import { Download, LockKeyhole } from "lucide-react";
import { Button, CopyButton } from "./ui";

export function RecoveryCodes({
  codes,
  onContinue,
  busy = false,
}: {
  codes: string[];
  onContinue(): void;
  busy?: boolean;
}) {
  const [acknowledged, setAcknowledged] = useState(false);
  function download() {
    const url = URL.createObjectURL(
      new Blob(
        [
          `Não se Meta no Brasil — códigos de recuperação\nCada código pode ser usado uma vez. Guarde em local privado.\n\n${codes.join("\n")}\n`,
        ],
        { type: "text/plain;charset=utf-8" },
      ),
    );
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "naosemetanobrasil-codigos-de-recuperacao.txt";
    anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 1_000);
  }
  return (
    <div className="recovery-panel">
      <div className="inline-heading">
        <LockKeyhole size={20} />
        <h2>Guarde seus códigos de recuperação</h2>
      </div>
      <p>
        Estes códigos aparecem somente agora. Você usará um deles se esquecer a
        senha. Cada código funciona uma vez.
      </p>
      <ul className="recovery-codes">
        {codes.map((code) => (
          <li key={code}>
            <code data-testid="recovery-code">{code}</code>
          </li>
        ))}
      </ul>
      <div className="button-row">
        <Button type="button" variant="secondary" onClick={download}>
          <Download size={16} />
          Baixar códigos
        </Button>
        <CopyButton value={codes.join("\n")} label="Copiar todos" />
      </div>
      <label className="checkbox-label">
        <input
          type="checkbox"
          checked={acknowledged}
          onChange={(event) => setAcknowledged(event.target.checked)}
        />
        Guardei meus códigos em um lugar seguro.
      </label>
      <Button
        type="button"
        disabled={!acknowledged}
        busy={busy}
        onClick={onContinue}
        className="full-width"
      >
        Continuar para meu perfil
      </Button>
    </div>
  );
}
