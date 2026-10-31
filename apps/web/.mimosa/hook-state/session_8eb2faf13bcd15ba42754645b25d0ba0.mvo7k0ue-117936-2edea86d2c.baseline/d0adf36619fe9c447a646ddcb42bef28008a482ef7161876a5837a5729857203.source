export function UtcTime({
  value,
  label,
  testId,
}: {
  value: string;
  label: string;
  testId?: string;
}) {
  return (
    <time
      dateTime={value}
      aria-label={label}
      title={value}
      data-testid={testId}
    >
      {value.replace('T', ' ').replace(/Z$/, ' UTC')}
    </time>
  );
}
