export default function DataTable({ rows }: { rows: any[] }) {
  return (
    <table>
      <tbody>
        {rows.map((r) => (
          <tr key={r.id}><td>{r.name}</td></tr>
        ))}
      </tbody>
    </table>
  )
}
