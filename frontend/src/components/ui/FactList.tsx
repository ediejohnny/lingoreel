/** FactList shows facts as label and value, in a box whose text can be selected and copied. */
export function FactList({ facts }: { facts: { label: string; value: string }[] }) {
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 rounded-control border border-line bg-surface-sunken p-3 text-xs select-text">
      {facts.map((fact) => (
        <div key={fact.label} className="contents">
          <dt className="text-fg-muted">{fact.label}</dt>
          <dd className="break-all text-fg">{fact.value}</dd>
        </div>
      ))}
    </dl>
  );
}
