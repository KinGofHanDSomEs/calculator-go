# Многопользовательский распределённый вычислитель арифметических выражений
Это калькулятор, выполняющий классические математические операции, например, +, -, *, /. В основе его создания стоят:
1. Персистентность - возможность программы восстанавливать свое состояние после перезагрузки.
2. Многопользовательский режим - возможность вычислять несколько выражений у разных пользователей.
3. Rest Api - архитектурный стиль взаимодействия компонентов распределенной системы, используется для передачи данных между сервером и пользователем.
4. GRPC - система удаленного вызова процедур, используется для вычисления задач, которые получаются в результате разделения выражения на операции.
## Схемы
### Схема разделения выражения
```mermaid
flowchart TD
    A[**1+2-3*4/5**] --> B(1+2)
    A --> C(3*4)
    C --> D(12/5)
    B --> E(3-2.4)
    D --> E
    E --> F{*0.6*}
```

### Схема работы приложения
```mermaid
flowchart TD
    GT{GetTask} --> Or
    PT{PostTask} --> Or
    Ag1[*Agent1*] -->|GRPC| GT
    GT --> Ag1
    Ag1 -->|GRPC| PT
    Ag2[*Agent2*] -->|GRPC| GT
    GT --> Ag2
    Ag2 -->|GRPC| PT
    Cl[**Client**]-->|HTTP| Au(Auth)
    Au -->|Token| Cl
    Cl -->|HTPP + Token| Or(Orchestrator)
    Or --> Cl
```
## Установка
1. Установите язык программирования [Golang](https://go.dev/dl/).
2. Установите текстовый редактор [Visual Studio Code](https://code.visualstudio.com/).
3. Установите систему контроля версий [Git](https://git-scm.com/downloads).
4. Создайте папку и откройте ее в Visual Studio Code.
5. В проекте слева нажмите на 4 квадратика - Extensions. В поле поиска введите go и скачайте первый модуль под названием Go.
6. Создайте клон репозитория с GitHub. Для этого в терминале Visual Studio Code введите следующую команду:
```
git clone https://github.com/kingofhandsomes/calculator-go
cd calculator-go
```
7. Зарегистрируйтесь и установите [Postman](https://www.postman.com/).
## Запуск приложения
1. Установите дополнительные пакеты:
```
go mod tidy
```
2. Пересоздайте базу данных:
```
go run storage/init/main.go
```
> [!TIP]
> Может возникнуть ошибка с gcc, для её решения установите и поместите его в системные переменные компьютера
3. Установите необходимую конфигурацию. По пути 'config/local.yaml' находится файл, в котором присутствуют следующие переменные:
- env - происхождение конфигурации;
- storage_path - путь, по которому находится хранилище данных;
- token_ttl - длительность jwt токена;
- TIME_ADDITION_MS - длительность вычисления сложения;
- TIME_SUBTRACTION_MS - длительность вычисления вычитания;
- TIME_MULTIPLICATIONS_MS - длительность вычисления умножения;
- TIME_DIVISIONS_MS - длительность вычисления деления;
- COMPUTING_POWER - количество агентов, которые будут асинхронно вычислять задачи;
- port - порт для Rest Api, то есть для работы пользователя с сервером;
- grpc_port - порт для gRPC, то есть для работы агентов с сервером.
4. Запустите приложение:
```
go run cmd/calculator/main.go --config="./config/local.yaml"
```
## Работа пользователя с сервером
1. **Регистрация:**  
![image](https://github.com/user-attachments/assets/b0813a08-66c8-433d-8d2a-e37429729b6c)
2. **Вход (аутентификация):**  
![image](https://github.com/user-attachments/assets/38af6a3f-803b-40f9-aa9a-ede02bf89a57)  
*Получаем токен. Для последующих запросов вставляем его в Headers - Authorization:*  
![image](https://github.com/user-attachments/assets/d8d7c961-e2c2-4a1c-978d-91ddb91d5ec8)  
> [!TIP]
> Данный токен содержит в себе ваши данные, срок его действия и является проводником для работы с оркестратором.
3. **Отправка выражения на вычисление:**  
![image](https://github.com/user-attachments/assets/3ab9eb7b-e936-475b-ac9f-c604a1ebc5b4)
4. **Вывод всех выражений:**  
![image](https://github.com/user-attachments/assets/0f314bcf-52bd-45c3-9672-aa5adb7def69)
5. **Вывод одного выражения:**
![image](https://github.com/user-attachments/assets/88285fbd-9924-47ab-9125-a14e421c8f90)
## Работа агентов с сервером
Для этого используется gRPC, создается сервер и клиент, в качестве сервера выступает оркестратор, в качестве клиента - агенты, которые получают задачи и асинхронно выполняют их. Пользователь не может выступать клиентом. Запросы:
- Запрос на получение задачи:  
![image](https://github.com/user-attachments/assets/a7934dbc-e0d5-4b36-912c-ec93f02da78a)
- Запрос на отправку решения задачи:
![image](https://github.com/user-attachments/assets/8b3e2ae1-40d9-422f-a190-5d12f5a42802)
## Вывод ошибок
1. **Register**
- *пустые поля login или password:*  
![image](https://github.com/user-attachments/assets/5a270ec2-73ed-4d74-84db-7e6832d1909c)
![image](https://github.com/user-attachments/assets/7f8ffb32-3c7b-4e9e-a58e-df465e531f18)
- *повторная отправка такого же login:*  
![image](https://github.com/user-attachments/assets/e6c03e43-51b9-4bcc-b8b2-3fd855dba7eb)
- *неверная json структура запроса:*
![image](https://github.com/user-attachments/assets/b297965c-7598-43bd-981d-7d8be3132244)
2. **Login**
- *пустые поля login или password:*
![image](https://github.com/user-attachments/assets/98119701-1d2c-4fcc-ba3b-5a23db7b0564)
![image](https://github.com/user-attachments/assets/d2619fea-e246-499c-af93-cda37d37683b)
- *неверный password:*  
![image](https://github.com/user-attachments/assets/a9654414-86ca-4547-995e-f1fdb211d0f7)
- *неверный login:*
![image](https://github.com/user-attachments/assets/ac5b57b0-ab01-439b-9648-11e5dc44a9dc)
- *неверная json структура запроса:*  
![image](https://github.com/user-attachments/assets/2fdcc79f-ba36-41af-9d8a-a7d825b22868)
3. **Calculate**
- *неверный токен:*  
![image](https://github.com/user-attachments/assets/f4630b1f-3dbd-45e3-9d25-04b18f684ead)
- *время действия токена истекло:*  
![image](https://github.com/user-attachments/assets/8fe0a322-0324-4028-a89d-7ba09af995ac)
- *неверное выражение:*  
![image](https://github.com/user-attachments/assets/2fd9a8f5-77c9-4805-a700-111dffb87336)
- *неверная json структура запроса:*  
![image](https://github.com/user-attachments/assets/af758a6b-a3b4-4687-9cfe-ec8c503f9f50)
4. **Expressions**
- *неверный токен:*  
![image](https://github.com/user-attachments/assets/71dd5e33-36dc-44a1-b061-d61e0b3c79cb)
- *время действия токена истекло:*  
![image](https://github.com/user-attachments/assets/b4bbfbad-edaf-434a-bfb3-2cba9b805c09)
5. **Expression**
- *неверный токен:*  
![image](https://github.com/user-attachments/assets/4d7a157f-df5f-4e76-9df8-8ca5ce3486f5)
- *время действия токена истекло:*  
![image](https://github.com/user-attachments/assets/cd824b33-43a1-4a2d-902f-4df38d97f9e8)
- *неверный id выражения:*
![image](https://github.com/user-attachments/assets/1caccb36-4cba-4343-83c2-32a707e88b71)
## Тесты
**Запуск тестов по отдельности:**
- *модульные:*
```
go test ./internal/transport/auth/test/auth_test.go
go test ./internal/transport/orchestrator/test/orchestrator_test.go
```
- *интеграционные:*
```
go test ./internal/app/test/app_test.go
```
**Запуск всех тестов сразу:**
```
go test ./...
```
## Связь с разработчиком
*Телеграмм:* **@KinGofHanDSomEs**
