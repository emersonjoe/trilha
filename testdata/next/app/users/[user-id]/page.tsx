export default async function User({ params }: { params: { "user-id": string } }) {
  const res = await fetch(`/api/users/${params["user-id"]}`)
  const user = await res.json()
  return <p>{user.name}</p>
}
