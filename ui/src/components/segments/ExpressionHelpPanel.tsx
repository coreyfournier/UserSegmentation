import styles from './ExpressionHelpPanel.module.css';

const BUILTIN = [
  'abs', 'ceil', 'floor', 'round', 'min', 'max', 'len',
  'sum', 'map', 'filter', 'all', 'any', 'count', 'find', 'none',
  'contains', 'startsWith', 'endsWith', 'trim', 'upper', 'lower',
];

const MATH_FNS = [
  { sig: 'exp(x)',     desc: "eˣ — Euler's number to the power x" },
  { sig: 'ln(x)',      desc: 'natural logarithm (base e)' },
  { sig: 'log2(x)',    desc: 'base-2 logarithm' },
  { sig: 'log10(x)',   desc: 'base-10 logarithm' },
  { sig: 'pow(x, y)', desc: 'x raised to the power y' },
  { sig: 'sin(x)',     desc: 'sine (x in radians)' },
  { sig: 'cos(x)',     desc: 'cosine (x in radians)' },
];

const EXAMPLES: Array<{
  note: string;
  ctx: string;
  exprs: Array<{ name: string; code: string }>;
}> = [
  {
    note: 'Weighted score',
    ctx: '{ Score: number, Weight: number }',
    exprs: [
      { name: 'Adjusted', code: 'abs(Score) * Weight' },
    ],
  },
  {
    note: 'Logistic sigmoid — log-odds → probability',
    ctx: '{ LogOdds: number }',
    exprs: [
      { name: 'P', code: '1.0 / (1.0 + exp(-LogOdds))' },
    ],
  },
  {
    note: 'Age-decayed signal sum → risk probability',
    ctx: '{ Signals: [{ weight, score, age_sec, tau_sec }], w0: number }',
    exprs: [
      { name: 'Z', code: 'w0 + sum(map(Signals, {.weight * .score * exp(-.age_sec / .tau_sec)}))' },
      { name: 'P', code: '1.0 / (1.0 + exp(-Z))' },
    ],
  },
  {
    note: 'Compliance max advance (50% of earned wages, capped by state)',
    ctx: '{ EarnedWages: number, StateCap: number }',
    exprs: [
      { name: 'MaxAllowed', code: 'min(EarnedWages * 0.5, StateCap)' },
    ],
  },
];

export default function ExpressionHelpPanel() {
  return (
    <details className={styles.panel}>
      <summary className={styles.summary}>
        ▸ Expression Reference
      </summary>
      <div className={styles.body}>
        <div className={styles.section}>
          <h4>Built-in functions</h4>
          <div className={styles.chips}>
            {BUILTIN.map((f) => (
              <span key={f} className={styles.chip}>{f}</span>
            ))}
          </div>
        </div>

        <div className={styles.section}>
          <h4>Math functions (registered by this service)</h4>
          <table className={styles.mathTable}>
            <tbody>
              {MATH_FNS.map(({ sig, desc }) => (
                <tr key={sig}>
                  <td>{sig}</td>
                  <td>{desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <a
            href="https://expr-lang.org/docs/language-definition"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.link}
          >
            Full language reference ↗
          </a>
        </div>

        <div className={styles.section}>
          <h4>Examples</h4>
          <div className={styles.examples}>
            {EXAMPLES.map(({ note, ctx, exprs }) => (
              <div key={note} className={styles.example}>
                <div className={styles.exampleNote}>{note}</div>
                <div className={styles.exampleCtx}>context: {ctx}</div>
                {exprs.map(({ name, code }) => (
                  <div key={name} className={styles.exprRow}>
                    <span className={styles.exprName}>{name} = </span>
                    {code}
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>
      </div>
    </details>
  );
}
