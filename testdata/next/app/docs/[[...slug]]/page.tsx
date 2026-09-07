export default function Docs({ params }: { params: { slug?: string[] } }) {
  return <p>{(params.slug ?? []).join("/")}</p>
}
