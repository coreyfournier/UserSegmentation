import type { EvaluateResponse, LayerStatus } from '../../api/types';
import styles from './ResultDisplay.module.css';

interface Props {
  result: EvaluateResponse;
}

const STRATEGY_COLORS: Record<string, string> = {
  static: '#3b82f6',
  rule: '#22c55e',
  percentage: '#8b5cf6',
  override: '#f97316',
  checklist: '#eab308',
};

const STATUS_COLORS: Record<LayerStatus, string> = {
  satisfied: '#22c55e',
  violated: '#ef4444',
  unevaluable: '#f97316',
  resolved: '#64748b',
  unresolved: '#64748b',
  skipped: '#f97316',
};

/**
 * Status is authoritative — never infer the outcome from failures.length. An
 * unevaluable gate has no failures precisely because it could not be judged.
 */
const STATUS_HINTS: Record<LayerStatus, string> = {
  satisfied: 'No problems found.',
  violated: 'One or more checks reported a problem.',
  unevaluable: 'Could not be judged — a dependency did not resolve, or a computed field failed.',
  resolved: 'Resolved to a segment.',
  unresolved: 'No segment matched.',
  skipped: 'A dependency did not resolve.',
};

export default function ResultDisplay({ result }: Props) {
  return (
    <div>
      <div className={styles.meta}>
        <span>Duration: <strong>{result.duration_us}us</strong></span>
      </div>

      {Object.entries(result.layers).map(([name, lr]) => (
        <div
          key={name}
          className={styles.card}
          style={{
            borderLeftColor:
              STATUS_COLORS[lr.status] ?? STRATEGY_COLORS[lr.strategy ?? ''] ?? '#64748b',
          }}
        >
          <div className={styles.layerName}>
            {name}
            <span
              title={STATUS_HINTS[lr.status]}
              style={{
                marginLeft: 8,
                fontSize: 11,
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.04em',
                color: STATUS_COLORS[lr.status] ?? '#64748b',
              }}
            >
              {lr.status}
            </span>
          </div>
          {(lr.segment || lr.strategy) && (
            <div className={styles.detail}>
              {lr.segment && <span>segment: <strong>{lr.segment}</strong></span>}
              {lr.strategy && <span>strategy: <strong>{lr.strategy}</strong></span>}
            </div>
          )}
          {lr.reason && <div className={styles.reason}>reason: {lr.reason}</div>}
          {lr.failures && lr.failures.length > 0 && (
            <ul style={{ margin: '8px 0 0', paddingLeft: 18 }}>
              {lr.failures.map((f) => (
                <li key={f.rule} style={{ fontSize: 12, marginBottom: 4 }}>
                  <code style={{ color: STATUS_COLORS.violated }}>{f.rule}</code>
                  {f.message && <span> — {f.message}</span>}
                  {f.messages && Object.keys(f.messages).length > 0 && (
                    <div className={styles.messages}>
                      {Object.entries(f.messages).map(([lang, text]) => (
                        <div key={lang} className={styles.message}>
                          <span className={styles.lang}>{lang}</span>
                          <span>{text}</span>
                        </div>
                      ))}
                    </div>
                  )}
                  {f.outputs && Object.keys(f.outputs).length > 0 && (
                    <div className={styles.outputs}>
                      {Object.entries(f.outputs).map(([k, v]) => (
                        <div key={k} className={styles.output}>
                          <code>{k}</code>
                          <span>{typeof v === 'object' && v !== null ? JSON.stringify(v) : String(v)}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </li>
              ))}
            </ul>
          )}
          {lr.computed && Object.keys(lr.computed).length > 0 && (
            <div className={styles.computed}>
              {Object.entries(lr.computed).map(([k, v]) => (
                <span key={k} className={styles.expr}>
                  {k}: <strong>{String(v)}</strong>
                </span>
              ))}
            </div>
          )}
          {lr.messages && Object.keys(lr.messages).length > 0 && (
            <div className={styles.messages}>
              {Object.entries(lr.messages).map(([lang, text]) => (
                <div key={lang} className={styles.message}>
                  <span className={styles.lang}>{lang}</span>
                  <span>{text}</span>
                </div>
              ))}
            </div>
          )}
          {lr.outputs && Object.keys(lr.outputs).length > 0 && (
            <div className={styles.outputs}>
              {Object.entries(lr.outputs).map(([k, v]) => (
                <div key={k} className={styles.output}>
                  <code>{k}</code>
                  <span>{typeof v === 'object' && v !== null ? JSON.stringify(v) : String(v)}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      ))}

      {result.warnings && result.warnings.length > 0 && (
        <div className={styles.warnings}>
          <h4>Warnings</h4>
          {result.warnings.map((w, i) => (
            <div key={i} className={styles.warning}>
              <strong>{w.segment}</strong>: {w.field} — {w.message}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
