/** 解析控制面 /metrics 暴露的 cp_* 文本行（无第三方 prom 解析）。 */

export type ParsedMetric = {
  name: string;
  labels: Record<string, string>;
  value: number;
};

function parseLabels(raw: string): Record<string, string> {
  const out: Record<string, string> = {};
  const re = /(\w+)="([^"]*)"/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(raw)) !== null) {
    out[m[1]] = m[2];
  }
  return out;
}

export function parsePrometheusSamples(text: string): ParsedMetric[] {
  const lines = text.split(/\n/);
  const out: ParsedMetric[] = [];
  for (const line of lines) {
    if (!line || line.startsWith("#")) continue;
    const idx = line.indexOf("{");
    if (idx === -1) {
      const parts = line.trim().split(/\s+/);
      if (parts.length >= 2) {
        const v = Number(parts[parts.length - 1]);
        if (!Number.isNaN(v)) out.push({ name: parts[0], labels: {}, value: v });
      }
      continue;
    }
    const end = line.indexOf("}", idx);
    if (end === -1) continue;
    const name = line.slice(0, idx);
    const labels = parseLabels(line.slice(idx, end + 1));
    const rest = line.slice(end + 1).trim();
    const parts = rest.split(/\s+/);
    if (parts.length < 1) continue;
    const value = Number(parts[parts.length - 1]);
    if (Number.isNaN(value)) continue;
    out.push({ name, labels, value });
  }
  return out;
}

export function workloadsByPhase(samples: ParsedMetric[]): Record<string, number> {
  const map: Record<string, number> = {};
  for (const s of samples) {
    if (s.name !== "cp_workloads_total") continue;
    const ph = s.labels.phase || "unknown";
    map[ph] = (map[ph] || 0) + s.value;
  }
  return map;
}

export function firstMetricValue(samples: ParsedMetric[], name: string): number | undefined {
  for (const s of samples) {
    if (s.name === name) return s.value;
  }
  return undefined;
}
