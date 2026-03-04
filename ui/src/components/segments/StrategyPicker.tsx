import type { StrategyType } from '../../api/types';
import styles from './StrategyPicker.module.css';

interface Props {
  value: StrategyType;
  onChange: (s: StrategyType) => void;
}

const options: { value: StrategyType; label: string }[] = [
  { value: 'static', label: 'Static' },
  { value: 'rule', label: 'Rule' },
  { value: 'percentage', label: 'Percentage' },
];

export default function StrategyPicker({ value, onChange }: Props) {
  return (
    <div className={styles.picker}>
      {options.map((o) => (
        <label key={o.value} className={`${styles.option} ${value === o.value ? styles.active : ''}`}>
          <input
            type="radio"
            name="strategy"
            value={o.value}
            checked={value === o.value}
            onChange={() => onChange(o.value)}
          />
          {o.label}
        </label>
      ))}
    </div>
  );
}
