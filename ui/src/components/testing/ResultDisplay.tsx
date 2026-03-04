import type { EvaluateResponse } from '../../api/types';
import styles from './ResultDisplay.module.css';

interface Props {
  result: EvaluateResponse;
}

const STRATEGY_COLORS: Record<string, string> = {
  static: '#3b82f6',
  rule: '#22c55e',
  percentage: '#8b5cf6',
  override: '#f97316',
};

export default function ResultDisplay({ result }: Props) {
  return (
    <div>
      <div className={styles.meta}>
        <span>User: <strong>{result.user_key}</strong></span>
        <span>Duration: <strong>{result.duration_us}us</strong></span>
      </div>

      {Object.entries(result.layers).map(([name, lr]) => (
        <div
          key={name}
          className={styles.card}
          style={{ borderLeftColor: STRATEGY_COLORS[lr.strategy] ?? '#64748b' }}
        >
          <div className={styles.layerName}>{name}</div>
          <div className={styles.detail}>
            <span>segment: <strong>{lr.segment}</strong></span>
            <span>strategy: <strong>{lr.strategy}</strong></span>
          </div>
          <div className={styles.reason}>reason: {lr.reason}</div>
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
