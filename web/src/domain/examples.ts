export const examples = [
  {
    id: "exemplo-1",
    category: "Conteúdo removido",
    platform: "Instagram",
    title: "Uma publicação deixou de estar disponível.",
    description:
      "Exemplo de como registrar uma remoção, anexar o aviso recebido e acompanhar uma eventual resposta.",
    kind: "removed",
    tag: "Documentação disponível",
  },
  {
    id: "exemplo-2",
    category: "Restrição de conta",
    platform: "Facebook",
    title: "Uma conta recebeu uma restrição.",
    description:
      "Exemplo de registro com data, contexto e a justificativa apresentada pela rede social.",
    kind: "restricted",
    tag: "Em análise",
  },
  {
    id: "exemplo-3",
    category: "Queda de alcance",
    platform: "Instagram",
    title: "Uma mudança de alcance precisa de contexto.",
    description:
      "Exemplo de relato que reúne métricas e períodos de comparação, sem atribuir uma causa automaticamente.",
    kind: "reach",
    tag: "Relato recebido",
  },
] as const;

export function filterExamples(query: string, platform: string) {
  const term = query.trim().toLocaleLowerCase("pt-BR");
  return examples.filter(
    (item) =>
      (platform === "Todas as redes" || item.platform === platform) &&
      `${item.title} ${item.description} ${item.category}`
        .toLocaleLowerCase("pt-BR")
        .includes(term),
  );
}
