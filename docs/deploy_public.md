# คู่มือ deploy API ผ่าน Cloudflare Tunnel

เนื้อหานี้เขียนเป็นภาษาไทย เพื่อให้คุณสามารถเรียก deploy ทั้งหมดได้ง่าย ๆ แบบคำสั่งเดียว และรู้ว่าแต่ละขั้นตอนทำอะไร

## การตั้งค่าก่อนเริ่ม

1. ตรวจสอบว่า `cloudflared login` เสร็จเรียบร้อย และไฟล์ `~/.cloudflared/config.yml` ชี้ไปที่ tunnel ของบัญชีคุณ (UUID จะตรงกับที่สร้างไว้ใหม่)
2. confirm ว่า DNS ของ `api.twentcode.com` ชี้ไปที่ tunnel (ใช้ `cloudflared tunnel route dns api-demo api.twentcode.com` ถ้าจำเป็น)
3. แก้ไขค่าที่จำเป็นใน `.env` เช่น DSN ของ Postgres/Redis ถ้ารันในพื้นที่ production

## คำสั่งเดียวจบ

เครื่องมือนี้เรียกว่า `scripts/deploy_public.sh` — รันแล้วจะ:

1. build binary และ image (`make docker-build`)
2. tag + push ขึ้น Docker Hub (ค่า default `kidpechcode/api_free_demo:latest`)
3. สร้าง network `demo-net` ถ้ายังไม่มี
4. เรียก Postgres และคอนเทนเนอร์ API ในเครือข่ายเดียวกัน (สคริปต์จะรอให้ Postgres พร้อมก่อน)
5. เปิด `cloudflared tunnel run api-demo` เพื่อให้ Cloudflare forward traffic มาที่ `localhost:8080`

### คำสั่ง (รันจาก root repo)

```bash
./scripts/deploy_public.sh
```

ถ้าต้องการใช้ registry อื่น เช่น `ghcr.io/kidpech`, ตั้ง ENV ก่อนรัน:

```bash
REGISTRY=ghcr.io/kidpech ./scripts/deploy_public.sh
```

หลังจากรันแล้ว ให้เปิด tab ใหม่ และเช็คว่า public endpoint ตอบ:

```bash
curl -i https://api.twentcode.com/api/v1/health
```

## รายละเอียดที่ script ทำให้

- **ขั้นตอน build+push**: สร้าง binary จากโค้ดปัจจุบัน และ push ไปยัง registry ที่กำหนด
- **Postgres + API**: เรียก container ใช้ environment เดียวกันกับที่ใช้ local เพื่อให้ API มีฐานข้อมูลพร้อม
- **Cloudflare Tunnel**: ใช้ tunnel ที่เคยสร้างไว้ (`api-demo`) เพื่อ expose `https://api.twentcode.com`

หากต้องการให้ tunnel ทำงานใน background:

```bash
./scripts/deploy_public.sh &> ~/deploy_public.log &
tail -f ~/deploy_public.log
```

จากนั้นหยุดด้วย `kill %1` หรือ `pkill -f cloudflared`

## คำแนะนำเพิ่มเติม

- ตรวจสอบว่าพอร์ต 8080 ไม่ถูกใช้งานก่อนรัน
- หากต้องการใช้ Redis จริง ให้เพิ่ม ENV ใน `.env` และรัน service ด้วย `docker run redis` ก่อน
- อย่าลืมล็อกอิน Cloudflare (`cloudflared login`) และ Docker Hub (`docker login`) ก่อนเริ่ม script

ต้องการให้ผมช่วยเขียนคู่มือเวอร์ชันภาษาอังกฤษ, หรือสร้าง task บน Makefile ให้เรียก script นี้ด้วยมั้ย?
