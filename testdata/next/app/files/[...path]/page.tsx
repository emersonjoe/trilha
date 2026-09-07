export default function File({ params }: { params: { path: string[] } }) {
  return <p>{params.path.join("/")}</p>
}
