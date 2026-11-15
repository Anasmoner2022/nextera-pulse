import urllib.request
import json

def get(path):
    with urllib.request.urlopen("http://localhost:8080" + path) as resp:
        return json.loads(resp.read().decode("utf-8"))

print("=== 1. Health Probe ===")
print(get("/health/live"))

print("\n=== 2. Readiness Probe (PostgreSQL Ping) ===")
print(get("/health/ready"))

print("\n=== 3. Available Cohorts ===")
print(get("/api/v1/cohorts"))

print("\n=== 4. Pilot Cohort 4 (Second Cohort) Health Summary (North Star KPI) ===")
c4_health = get("/api/v1/cohorts/4/health")
print(json.dumps(c4_health, indent=2))

print("\n=== 5. Filter Students: Status = PAUSED ===")
paused = get("/api/v1/students?cohortId=4&status=PAUSED")
print("Paused count:", len(paused["data"]))
for p in paused["data"]:
    print(f"  - {p['login']} (sourceUserId: {p['sourceUserId']}): Status={p['pace']['status']}, Reasons={p['pace']['reasons']}")

print("\n=== 6. Sample Student Payload for Ziad's Frontend ===")
students = get("/api/v1/students?cohortId=4")
print("Total Cohort 4 students:", len(students["data"]))
print("First student payload:")
print(json.dumps(students["data"][0], indent=2))


print("\n=== 7. Search Filter: search=hazem ===")
search_res = get("/api/v1/students?search=hazem")
print("Search results:", len(search_res["data"]))
for s in search_res["data"]:
    print(f"  - {s['login']}: {s['firstName']} {s['lastName']} (Level {s['pace']['actualLevel']}, Status: {s['pace']['status']})")
